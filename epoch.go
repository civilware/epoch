package epoch

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"math"
	"math/big"
	"net"
	"net/url"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/civilware/tela/logger"
	"github.com/deroproject/derohe/astrobwt/astrobwtv3"
	"github.com/deroproject/derohe/block"
	"github.com/deroproject/derohe/blockchain"
	"github.com/deroproject/derohe/globals"
	"github.com/deroproject/derohe/rpc"
	"github.com/gorilla/websocket"
)

// EPOCH: Event-Driven Propagation of Crowd Hashing (EPOCH)

// Web socket connection and sync
type connection struct {
	ws *websocket.Conn
	sync.Mutex
}

// DERO block template and sync
type jobs struct {
	job rpc.GetBlockTemplate_Result
	sync.RWMutex
}

// EPOCH main structure
type EPOCH struct {
	conn       connection             // Connection to GetWork from DERO node
	jobs       jobs                   // DERO block template for work
	port       string                 // GetWork port that EPOCH will connect to
	address    string                 // EPOCH reward address
	processing bool                   // When EPOCH is processing or submitting jobs
	maxHashes  int                    // maxHashes is the maximum accepted hashes for a single request, this can be set as per the host app with EPOCH package defining a hard limit of LIMIT_MAX_HASHES
	maxThreads int                    // maxThreads is the maximum concurrent workers
	semaphore  chan struct{}          // Limit EPOCH workers to maxThreads
	session    GetSessionEPOCH_Result // session counts the total hashes and submissions that have occurred while connection is active
	sync.RWMutex
}

var epoch EPOCH

const (
	DEFAULT_MAX_THREADS = 2     // Default max thread value for EPOCH
	DEFAULT_WORK_PORT   = 10100 // Default DERO GetWork port
	LIMIT_MAX_HASHES    = 10000 // Maximum value that EPOCH package will accept hashes per request at
)

// Initialize EPOCH package defaults
func init() {
	epoch.port = fmt.Sprintf(":%d", DEFAULT_WORK_PORT)
	SetMaxThreads(DEFAULT_MAX_THREADS)
	epoch.maxHashes = 1000
}

// Check if EPOCH connection is active
func IsActive() bool {
	return epoch.conn.ws != nil
}

// Set EPOCH processing when doing jobs or submissions
func setProcessing(b bool) {
	epoch.Lock()
	epoch.processing = b
	epoch.Unlock()
}

// Set a new DERO block template and return lastError
func (e *EPOCH) newJob(job rpc.GetBlockTemplate_Result) (lastError string) {
	e.jobs.Lock()
	e.jobs.job = job
	e.jobs.Unlock()

	lastError = job.LastError

	return
}

// Get the current DERO block template
func (e *EPOCH) getJob() (job rpc.GetBlockTemplate_Result) {
	e.jobs.RLock()
	job = e.jobs.job
	e.jobs.RUnlock()

	return
}

// JobIsReady waits for a JobID to be present, it returns error if job is not found before timeout duration
func JobIsReady(timeout time.Duration) (err error) {
	timer := time.NewTicker(timeout)
	defer timer.Stop()

	for {
		select {
		case <-timer.C:
			err = fmt.Errorf("could not get EPOCH job after %s", timeout)
			return
		default:
			if epoch.jobs.job.JobID != "" {
				return
			}

			time.Sleep(time.Second)
		}
	}
}

// Check if EPOCH is processing jobs or submissions
func IsProcessing() bool {
	epoch.RLock()
	defer epoch.RUnlock()

	return epoch.processing
}

// Set the EPOCH reward address, must be a registered DERO address
func SetAddress(address string) (err error) {
	_, err = globals.ParseValidateAddress(address)
	if err != nil {
		return
	}

	epoch.Lock()
	epoch.address = address
	epoch.Unlock()

	return
}

// Get the EPOCH reward address
func GetAddress() string {
	epoch.RLock()
	defer epoch.RUnlock()

	return epoch.address
}

// Set the GetWork port if port is valid
func SetPort(port int) (err error) {
	if port < 1 || port > 65535 {
		err = fmt.Errorf("invalid EPOCH port")
		return
	}

	epoch.Lock()
	epoch.port = fmt.Sprintf(":%d", port)
	epoch.Unlock()

	return
}

// Get the EPOCH work port
func GetPort() string {
	epoch.RLock()
	defer epoch.RUnlock()

	return strings.Trim(epoch.port, ":")
}

// Set the max amount of hash attempts or job submissions that a single request can handle, exceeding MAX_HASHES will return error
func SetMaxHashes(i int) (err error) {
	if i > LIMIT_MAX_HASHES {
		err = fmt.Errorf("cannot exceed %d hashes", LIMIT_MAX_HASHES)
		return
	}

	epoch.Lock()
	epoch.maxHashes = i
	epoch.Unlock()

	return
}

// Get the EPOCH maxHashes value
func GetMaxHashes() int {
	epoch.RLock()
	defer epoch.RUnlock()

	return epoch.maxHashes
}

// Parse EPOCH hashes and return as formatted string
func HashesToString(hashes uint64) string {
	if hashes > 10000000 {
		return fmt.Sprintf("%.1fM", float64(hashes)/1000000)
	} else {
		return fmt.Sprintf("%.1fK", float64(hashes)/1000)
	}
}

// Set the max amount of threads to be used when attempting or submitting, max is limited to total available and minimum of 1
func SetMaxThreads(i int) {
	max := runtime.NumCPU()
	if i > max {
		i = max
	} else if i < 1 {
		i = 1
	}

	epoch.Lock()
	epoch.maxThreads = i
	epoch.Unlock()
}

// Get the EPOCH maxThreads value
func GetMaxThreads() int {
	epoch.RLock()
	defer epoch.RUnlock()

	return epoch.maxThreads
}

// Stop listening to GetWork server
func StopGetWork() {
	if IsActive() {
		epoch.conn.ws.Close()
		epoch.conn.ws = nil
	}
}

// Start listening to GetWork server, if address is empty string epoch.address will be used
// endpoint is a DERO daemon address and will use the port defined by SetPort() to connect to GetWork,
// when StartGetWork is successfully connected it will set the EPOCH session totals to zero
func StartGetWork(address, endpoint string) (err error) {
	if IsActive() {
		err = fmt.Errorf("already running")
		return
	}

	host, _, err := net.SplitHostPort(endpoint)
	if err != nil {
		err = fmt.Errorf("could not get host: %s", err)
		return
	}

	if address != "" {
		err = SetAddress(address)
		if err != nil {
			err = fmt.Errorf("could not set address: %s", err)
			return
		}
	}

	_, err = globals.ParseValidateAddress(epoch.address)
	if err != nil {
		err = fmt.Errorf("address %q is not valid: %s", epoch.address, err)
		return
	}

	endpoint = host + epoch.port

	u := url.URL{Scheme: "wss", Host: endpoint, Path: "/ws/" + epoch.address}

	dialer := websocket.DefaultDialer
	dialer.TLSClientConfig = &tls.Config{
		InsecureSkipVerify: true,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	epoch.conn.ws, _, err = websocket.DefaultDialer.DialContext(ctx, u.String(), nil)
	if err != nil {
		epoch.conn.ws = nil
		return
	}

	logger.Printf("[EPOCH] Connected to %s\n", u.String())
	logger.Printf("[EPOCH] Will use %d threads\n", epoch.maxThreads)

	epoch.session.Hashes = 0
	epoch.session.MiniBlocks = 0
	epoch.semaphore = make(chan struct{}, epoch.maxThreads)

	go func() {
		defer StopGetWork()
		var result rpc.GetBlockTemplate_Result
		for {
			if err = epoch.conn.ws.ReadJSON(&result); err != nil {
				if !strings.Contains(err.Error(), "closed network connection") {
					logger.Errorf("[EPOCH] connection error: %s\n", err)
				}
				break
			}

			if lastError := epoch.newJob(result); lastError != "" {
				logger.Errorf("[EPOCH] Job error: %s\n", epoch.jobs.job.LastError)
			}
		}

		logger.Printf("[EPOCH] Closed\n")
	}()

	return
}

// GetSession returns the current EPOCH session statistics, it will wait while EPOCH is processing and return error if result is not found before timeout duration
func GetSession(timeout time.Duration) (session GetSessionEPOCH_Result, err error) {
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	for {
		select {
		case <-timer.C:
			err = fmt.Errorf("could not get EPOCH session after %s", timeout)
			return
		default:
			if !IsProcessing() {
				session = epoch.session
				return
			}

			time.Sleep(time.Millisecond * 25)
		}
	}
}

// Compute POW hash from a job template and return variables for block submission
func powHash() (job rpc.GetBlockTemplate_Result, powhash [32]byte, work [block.MINIBLOCK_SIZE]byte, diff big.Int, err error) {
	var random_buf [12]byte
	rand.Read(random_buf[:])

	// nonce_buf := work[block.MINIBLOCK_SIZE-5:] // since slices are linked, it modifies parent

	job = epoch.getJob()

	n, err := hex.Decode(work[:], []byte(job.Blockhashing_blob))
	if err != nil || n != block.MINIBLOCK_SIZE {
		err = fmt.Errorf("block hashing could not be decoded successfully %+v %d %v", job, n, err)
		return
	}

	copy(work[block.MINIBLOCK_SIZE-12:], random_buf[:]) // add more randomization in the mix
	work[block.MINIBLOCK_SIZE-1] = byte(1)

	diff.SetString(job.Difficulty, 10)

	if work[0]&0xf != 1 { // check  version
		err = fmt.Errorf("unknown version, please check for updates %v", work[0]&0x1f)
		return
	}

	// binary.BigEndian.PutUint32(nonce_buf, uint32(1))

	powhash = astrobwtv3.AstroBWTv3(work[:])

	return
}

// Check if powhash is valid and submit it as a miniblock to connected daemon if so
func submitBlock(job rpc.GetBlockTemplate_Result, powhash [32]byte, work [block.MINIBLOCK_SIZE]byte, diff big.Int) (valid bool, err error) {
	if !IsActive() {
		err = fmt.Errorf("connection is closed")
		return
	}

	if blockchain.CheckPowHashBig(powhash, &diff) { // note we are doing a local, NW might have moved meanwhile
		logger.Printf("[EPOCH] Submitting valid miniblock POW hash, difficulty: %s height: %d\n", job.Difficulty, job.Height)
		epoch.conn.Lock()
		defer epoch.conn.Unlock()
		if err = epoch.conn.ws.WriteJSON(rpc.SubmitBlock_Params{JobID: job.JobID, MiniBlockhashing_blob: fmt.Sprintf("%x", work[:])}); err == nil {
			valid = true
		}
	}

	return
}

// AttemptHashes preforms the POW for the number of hashes and submits valid hashes as miniblocks to the connected node,
// when it is called it increases the session total for hashes and blocks as per the result
func AttemptHashes(hashes int) (result EPOCH_Result, err error) {
	if !IsActive() {
		err = fmt.Errorf("epoch is not active")
		return
	}

	if hashes > GetMaxHashes() {
		err = fmt.Errorf("hashes exceeds maxHashes %d/%d", hashes, epoch.maxHashes)
		return
	}

	setProcessing(true)
	defer setProcessing(false)

	var wg sync.WaitGroup

	i := 0
	now := time.Now()

	for i = 0; i < hashes; i++ {
		if result.Error != nil {
			break
		}

		epoch.semaphore <- struct{}{}

		wg.Add(1)
		go func() {
			defer func() {
				<-epoch.semaphore
				wg.Done()
			}()

			job, powhash, work, diff, err := powHash()
			if err != nil {
				result.Error = err
				return
			}

			valid, err := submitBlock(job, powhash, work, diff)
			if err != nil {
				result.Error = err
				return
			}

			if valid {
				result.Submitted++
			}
		}()
	}

	wg.Wait()

	duration := time.Since(now)
	result.Duration = duration.Milliseconds()

	h := uint64(i)
	result.Hashes = h
	epoch.session.Hashes += h
	epoch.session.MiniBlocks += result.Submitted
	hashPerSecond := float64(h) / duration.Seconds()
	result.HashPerSec = math.Round(hashPerSecond*100) / 100

	return
}

// SubmitHashes checks and submits valid pre computed hashes as miniblocks to the connected node,
// only the block session total will be increased when it is called
func SubmitHashes(params []Submit_Params) (result EPOCH_Result, err error) {
	if !IsActive() {
		err = fmt.Errorf("epoch is not active")
		return
	}

	l := len(params)
	if l > GetMaxHashes() {
		err = fmt.Errorf("requested submission exceeds maxHashes %d/%d", epoch.maxHashes, l)
		return
	}

	setProcessing(true)
	defer setProcessing(false)

	var wg sync.WaitGroup

	i := 0
	now := time.Now()

	for _, p := range params {
		if result.Error != nil {
			break
		}

		epoch.semaphore <- struct{}{}

		wg.Add(1)
		go func(p Submit_Params) {
			defer func() {
				<-epoch.semaphore
				wg.Done()
			}()

			valid, err := submitBlock(p.Job, p.PowHash, p.EpochWork, p.Difficulty)
			if err != nil {
				result.Error = err
				return
			}

			i++
			if valid {
				result.Submitted++
			}
		}(p)
	}

	wg.Wait()

	result.Duration = time.Since(now).Milliseconds() // result will likely be in µs so 0
	result.Hashes = uint64(i)

	epoch.session.MiniBlocks += result.Submitted

	return
}
