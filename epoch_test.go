package epoch

import (
	"context"
	"encoding/hex"
	"math/big"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/deroproject/derohe/block"
	"github.com/deroproject/derohe/cryptography/crypto"
	"github.com/deroproject/derohe/globals"
	"github.com/deroproject/derohe/rpc"
	"github.com/deroproject/derohe/walletapi"
	"github.com/stretchr/testify/assert"
)

// This test requires a simulator with a running GetWork server

// Test EPOCH package
func TestEPOCH(t *testing.T) {
	testPath := "epoch_tests"
	walletName := "epoch_sim"
	walletSeed := "193faf64d79e9feca5fce8b992b4bb59b86c50f491e2dc475522764ca6666b6b"
	walletPath := filepath.Join(testPath, walletName)

	// Cleanup directories used for package test
	t.Cleanup(func() {
		StopGetWork()
		os.RemoveAll(testPath)
	})

	os.RemoveAll(testPath)

	endpoint := "127.0.0.1:20000"
	globals.Arguments["--testnet"] = true
	globals.Arguments["--simulator"] = true
	globals.Arguments["--daemon-address"] = endpoint
	globals.InitNetwork()

	// Create simulator wallet to use for rewards
	w, err := createTestWallet(walletName, walletPath, walletSeed)
	if err != nil {
		t.Fatalf("Failed to create wallet: %s", err)
	}

	// Connect simulator wallet
	if err := walletapi.Connect(endpoint); err != nil {
		t.Fatalf("Failed to connect wallet to simulator: %s", err)
	}

	// Test SetPort
	t.Run("SetPort", func(t *testing.T) {
		// Test invalid port
		err = SetPort(9999999)
		assert.Error(t, err, " Invalid port should error")

		// Set EPOCH to default GetWork port
		err = SetPort(DEFAULT_WORK_PORT)
		if err != nil {
			t.Fatalf("Failed to set port: %s", err)
		}

		assert.Equal(t, strconv.Itoa(DEFAULT_WORK_PORT), GetPort(), "Work port should be equal")
	})

	// Test SetAddress
	t.Run("SetAddress", func(t *testing.T) {
		// Test invalid address
		err = SetAddress("invalid")
		assert.Error(t, err, " Invalid address should error")

		// Set EPOCH address to created simulator wallet
		err = SetAddress(w.GetAddress().String())
		if err != nil {
			t.Fatalf("Failed to set wallet address: %s", err)
		}
		assert.Equal(t, w.GetAddress().String(), epoch.address, "Addresses should be equal")
	})

	// Test SetMaxThreads
	t.Run("SetMaxThreads", func(t *testing.T) {
		// Try to set more than available
		cpuMax := runtime.NumCPU()
		SetMaxThreads(cpuMax + 1)
		assert.Equal(t, cpuMax, epoch.maxThreads, "Max available should be highest thread setting")

		// Try to set none
		SetMaxThreads(0)
		assert.Equal(t, 1, epoch.maxThreads, "Min thread should be one")

		// Set to default
		maxThreads := 2
		SetMaxThreads(maxThreads)
		assert.Equal(t, maxThreads, GetMaxThreads(), "SetMaxThreads should be %d", maxThreads)
	})

	t.Run("Offline", func(t *testing.T) {
		// Hashes when server is offline
		_, err = AttemptHashes(1)
		assert.Error(t, err, "AttemptHashes should error when offline")
		_, err = SubmitHashes([]Submit_Params{})
		assert.Error(t, err, "SubmitHashes should error when offline")
		// Call methods when offline
		_, err = GetMaxHashesEPOCH(context.Background())
		assert.Error(t, err, "GetMaxHashes should error when offline")
		_, err = GetAddressEPOCH(context.Background())
		assert.Error(t, err, "GetAddressEPOCH should error when offline")
		assert.False(t, IsProcessing(), "Should not be processing when offline")
		_, err = GetSessionEPOCH(context.Background())
		assert.Error(t, err, "GetSessionEPOCH should error when offline")
		_, err = submitBlock(rpc.GetBlockTemplate_Result{}, [32]byte{}, [block.MINIBLOCK_SIZE]byte{}, big.Int{})
		assert.Error(t, err, "submitBlock should error when offline")
		// powHash error
		epoch.jobs.job.Blockhashing_blob = "invalid" // won't decode
		_, _, _, _, err = powHash()
		assert.Error(t, err, "powHash should error when offline")
		// HashesToString
		thousandFormat := uint64(10100)
		millionFormat := uint64(10100000)
		assert.Equal(t, "10.1K", HashesToString(thousandFormat), "HashesToString thousand format should be equal")
		assert.Equal(t, "10.1M", HashesToString(millionFormat), "HashesToString million format should be equal")
		// Timeout JobIsReady
		err = JobIsReady(time.Second)
		assert.Error(t, err, "JobIsReady should error on timeout")
		// Timeout GetSession
		setProcessing(true)
		_, err = GetSession(time.Second)
		assert.Error(t, err, "GetSession should error on timeout")
		setProcessing(false)
	})

	// Start GetWork server and sync balance
	err = StartGetWork("", endpoint)
	assert.NoError(t, err, "Starting EPOCH should not error: %s", err)

	w.Sync_Wallet_Memory_With_Daemon()

	err = JobIsReady(time.Second * 10) // wait for connection and jobs
	assert.NoError(t, err, "Finding job should not error: %s", err)

	if !IsActive() {
		t.Fatalf("Not connected to %s GetWork", endpoint)
	}

	// Test MaxHashes
	t.Run("MaxHashes", func(t *testing.T) {
		// A valid max hash value
		maxHashes := 10
		err := SetMaxHashes(maxHashes)
		assert.NoError(t, err, "SetMaxHashes should not error: %s", err)

		// Attempt above max hash
		_, err = AttemptHashes(maxHashes + 1)
		assert.Error(t, err, "Attempting above maxHashes should error")

		params := []Submit_Params{}
		for i := 0; i < maxHashes+2; i++ {
			params = append(params, Submit_Params{})
		}

		_, err = SubmitHashes(params)
		assert.Error(t, err, "SubmitHashes above maxHashes should error")

		// Test package limit
		err = SetMaxHashes(LIMIT_MAX_HASHES + 1)
		assert.Error(t, err, "SetMaxHashes should error above %d hashes", LIMIT_MAX_HASHES)

		// A valid max hash value
		maxHashes = 1000
		err = SetMaxHashes(maxHashes)
		assert.NoError(t, err, "SetMaxHashes should not error: %s", err)
	})

	hashes := []int{5, 25, 100} // Test these hash amounts
	startBalance, _ := w.Get_Balance()
	lastHeight := uint64(0)
	submitted := false

	// Test AttemptEPOCH
	t.Run("AttemptEPOCH", func(t *testing.T) {
		for _, h := range hashes {
			lastHeight = w.Get_Daemon_Height()
			t.Logf("Starting %d hashes at height %d", h, lastHeight)

			res, err := AttemptEPOCH(context.Background(), Attempt_Params{Hashes: h})
			assert.NoError(t, err, "AttemptEPOCH should not error: %s", err)

			t.Logf("Height: %d %+v", w.Get_Daemon_Height(), res)
			if res.Submitted > 0 {
				submitted = true
			}
		}
	})

	t.Run("ConcurrentAttemptEPOCH", func(t *testing.T) {
		lastHeight = w.Get_Daemon_Height()

		var wg sync.WaitGroup
		for i := 0; i < 3; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				res, err := AttemptEPOCH(context.Background(), Attempt_Params{Hashes: hashes[1]})
				assert.NoError(t, err, "AttemptEPOCH should not error: %s", err)
				if res.Submitted > 0 {
					submitted = true
				}
			}()
		}
		wg.Wait()
	})

	// Test SubmitEPOCH
	t.Run("SubmitEPOCH", func(t *testing.T) {
		for _, h := range hashes {
			params := []Submit_Params{}
			for i := 0; i < h; i++ {
				job, pow, work, diff, err := powHash()
				assert.NoError(t, err, "powHash should not error: %s", err)
				params = append(params,
					Submit_Params{
						Job:        job,
						PowHash:    pow,
						EpochWork:  work,
						Difficulty: diff,
					},
				)
			}

			lastHeight = w.Get_Daemon_Height()
			t.Logf("Starting %d hashes at height %d", h, lastHeight)

			res, err := SubmitEPOCH(context.Background(), params)
			assert.NoError(t, err, "SubmitEPOCH should not error: %s", err)

			t.Logf("Height: %d %+v", w.Get_Daemon_Height(), res)
			if res.Submitted > 0 {
				submitted = true
			}
		}
	})

	// Test other methods
	t.Run("Methods", func(t *testing.T) {
		handler := GetHandler()
		for n := range handler {
			assert.Contains(t, n, "EPOCH", "Methods should have EPOCH suffix")
		}

		setProcessing(true)
		go func() { // delay GetSessions response from processing
			time.Sleep(time.Millisecond * 100)
			setProcessing(false)
		}()
		sess, err := GetSessionEPOCH(context.Background())
		assert.NoError(t, err, "GetSessionEPOCH should not error: %s", err)
		assert.NotZero(t, sess.Hashes, "Hashes should be above zero")

		// GetMaxHashes
		_, err = GetMaxHashesEPOCH(context.Background())
		assert.NoError(t, err, "GetMaxHashes should not error: %s", err)

		// GetAddressEPOCH
		res, err := GetAddressEPOCH(context.Background())
		assert.NoError(t, err, "GetAddressEPOCH should not error: %s", err)
		assert.Equal(t, w.GetAddress().String(), res.Address, "Addresses should be equal")
	})

	// Check balance if submitted block
	if submitted {
		for lastHeight >= w.Get_Daemon_Height() {
			time.Sleep(time.Second)
		}

		w.Sync_Wallet_Memory_With_Daemon()
		endBalance := w.GetAccount().Balance_Mature
		assert.Greater(t, endBalance, startBalance, "Balance should be greater")
		t.Logf("Start balance: %d  End balance: %d  Height: %d", startBalance, endBalance, w.Get_Daemon_Height())
	}

	// Test errors
	t.Run("Errors", func(t *testing.T) {
		// Already running try to start again
		err := StartGetWork("", "")
		assert.Error(t, err, "StartGetWork should error when running already")

		// Start server with invalid endpoint string
		StopGetWork()
		err = StartGetWork("", "")
		assert.Error(t, err, "StartGetWork should error with invalid endpoint")

		// Start server with invalid address
		err = StartGetWork("invalid", endpoint)
		assert.Error(t, err, "StartGetWork should error with invalid address param")

		// Invalid epoch.address
		epoch.address = ""
		err = StartGetWork("", endpoint)
		assert.Error(t, err, "StartGetWork should error with invalid epoch.address")

		// Invalid daemon address
		epoch.address = w.GetAddress().String()
		err = StartGetWork("", "invalid:20101") // should err on invalid host
		assert.Error(t, err, "StartGetWork should error with invalid epoch.address")
	})
}

// Test EPOCH at its full capacity against a mainnet node, test must have RUN_MAINNET_TEST=true argument or this will be skipped
func TestMainnet(t *testing.T) {
	if os.Getenv("RUN_MAINNET_TEST") != "true" {
		t.Skipf("Use %q to run mainnet test", "RUN_MAINNET_TEST=true go test -run TestMainnet -v")
	}

	endpoint := "89.38.99.117:10102" // DERO mainnet remote default
	globals.Arguments["--testnet"] = false
	globals.Arguments["--simulator"] = false
	globals.Arguments["--daemon-address"] = endpoint
	globals.InitNetwork()
	address := "dero1qy0khp9s9yw2h0eu20xmy9lth3zp5cacmx3rwt6k45l568d2mmcf6qgcsevzx" // Artificer
	t.Cleanup(StopGetWork)

	// Use less then available
	SetMaxThreads(runtime.NumCPU() - 1)

	err := StartGetWork(address, endpoint)
	assert.NoError(t, err, "Starting EPOCH should not error: %s", err)

	err = JobIsReady(time.Second * 10) // wait for connection and jobs
	assert.NoError(t, err, "Finding job should not error: %s", err)

	if !IsActive() {
		t.Fatalf("Not connected to GetWork")
	}

	var wg sync.WaitGroup
	run := 30
	hashes := 1000
	start := time.Now()

	for i := 0; i < run; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := AttemptHashes(hashes)
			if err != nil {
				t.Errorf("AttemptHashes: %s", err)
			}
		}()
	}

	wg.Wait()

	took := time.Since(start)
	total := hashes * run

	t.Logf("Took %dms for %d hashes, [%.0fH/s]", took.Milliseconds(), total, float64(total)/took.Seconds())
}

// Benchmark for AttemptHashes with default settings against a simulator node
func BenchmarkAttemptHashes(b *testing.B) {
	endpoint := "127.0.0.1:20000"
	globals.Arguments["--testnet"] = true
	globals.Arguments["--simulator"] = true
	globals.Arguments["--daemon-address"] = endpoint
	globals.InitNetwork()
	address := "deto1qyre7td6x9r88y4cavdgpv6k7lvx6j39lfsx420hpvh3ydpcrtxrxqg8v8e3z"
	b.Cleanup(StopGetWork)

	if err := StartGetWork(address, endpoint); err != nil {
		b.Fatalf("StartGetWork error: %s", err)
	}

	err := JobIsReady(time.Second * 10) // wait for connection and jobs
	if err != nil {
		b.Fatalf("Finding job should not error: %s", err)
	}

	if !IsActive() {
		b.Fatalf("Not connected to GetWork")
	}

	hashes := 100

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := AttemptHashes(hashes)
		if err != nil {
			b.Fatalf("AttemptHashes failed: %s", err)
		}
	}
	b.StopTimer()
}

// Create test wallet for simulator
func createTestWallet(name, dir, seed string) (wallet *walletapi.Wallet_Disk, err error) {
	seed_raw, err := hex.DecodeString(seed)
	if err != nil {
		return
	}

	err = os.MkdirAll(dir, os.ModePerm)
	if err != nil {
		return
	}

	filename := filepath.Join(dir, name)

	os.Remove(filename)

	wallet, err = walletapi.Create_Encrypted_Wallet(filename, "", new(crypto.BNRed).SetBytes(seed_raw))
	if err != nil {
		return
	}

	wallet.SetNetwork(false)
	wallet.SetOnlineMode()
	wallet.Save_Wallet()

	return
}
