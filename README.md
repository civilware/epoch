# EPOCH: Event-driven Propagation of Crowd Hashing

**Crowd Mining** is an innovative monetization protocol designed to transform how game and application developers fund their projects and reward their users. By leveraging the collective computing power of users, EPOCH allows developers to move away from traditional monetization methods, such as gathering and selling user data or embedding advertisements into applications. Instead, users generate mining hashes through their interactions with the application, which are then submitted to a DERO node, potentially rewarding decentralized application creators through mining rewards.

#### Crowd Mining: Inspired by Collaborative Computing

The concept of Crowd Mining draws inspiration from distributed computing projects like Folding@home, where individuals contribute their computing power to solve complex scientific problems. Just as Folding@home harnesses the power of a global network of volunteers to advance medical research, Crowd Mining leverages the collective computing power of users to support decentralized applications.

#### Key Features of Crowd Mining:

1. **User-Generated Mining Hashes**: Users contribute to the application’s mining efforts through their interactions, generating mining hashes that support decentralized application creators.

2. **Decentralized Rewards**: Mining hashes are submitted to a DERO node, leading to potential mining rewards for the creators of decentralized applications.

3. **Enhanced User Engagement**: Users are incentivized to engage more deeply with applications, as their interactions directly contribute to unlocking in-game or in-app features and content.

4. **Privacy and Experience**: Unlike traditional monetization methods used by major technology companies like Facebook, Google, and Amazon, which profit from collecting and selling user data, Crowd Mining maintains user privacy and enhances the overall user experience by eliminating intrusive advertisements.

5. **Examples of Application**: 
   - **Mobile Games**: Players generate mining hashes by progressing through the game, unlocking exclusive levels, characters, or in-game currency.
   - **Productivity Apps**: Users gain access to premium features by generating mining hashes through regular app use.

6. **Unobtrusive and Scalable**: Crowd Mining offers unobtrusive rewards for all parties involved, scaling with the quality and engagement of the end product. The system promotes network and economic decentralization through the efficient and accessible mining algorithm AstroBWT, allowing CPUs to easily generate mining hashes.

#### Synergy of Quality and Mining
EPOCH's Crowd Mining protocol incentivizes developers to create high-quality user experiences and content. The generation of mining hashes is directly tied to user engagement, meaning that developers must focus on delivering compelling, enjoyable, and valuable applications to encourage continued user participation.

By enhancing the quality of their applications, developers can attract more users and encourage existing users to spend more time interacting with the application. This increased interaction leads to the generation of more mining hashes, which in turn translates to greater potential mining rewards. Consequently, developers who invest in better user experiences and content are more likely to see higher engagement and greater rewards from the Crowd Mining protocol.

This relationship fosters a virtuous cycle:

- Better applications attract and retain users.
- Increased user engagement generates more mining hashes.
- More mining hashes lead to higher rewards for developers.
- Higher rewards incentivize further investment in application quality.

By aligning the interests of developers and users, EPOCH's Crowd Mining protocol ensures that the focus remains on creating superior applications that benefit all parties involved.

### Package Information
The `civilware/epoch` [go package](https://pkg.go.dev/github.com/Civilware/epoch) provides a interface for applications connecting to a [DERO](https://dero.io) node's GetWork server. It is designed to handle incoming EPOCH requests, enabling users to attempt or submit mining hashes to earn network rewards. This package can be utilized directly in applications or through WebSocket (WS) connections using the provided API, making it versatile for various application use cases.

### Importing the Package
Before using the package, ensure [go](https://go.dev/doc/install) is installed on your device. You can import the `civilware/epoch` package in your Go file with the following statement:
```go
import "github.com/civilware/epoch"
```

### Examples Using the Package Directly
To effectively use the `civilware/epoch` package, your application must connect to a DERO node. Ensure that your network configurations are correctly set for the DERO network. Below is an example demonstrating how to set up a connection and utilize the package's main functionalities:
```go
package main

import (
	"fmt"
	"time"

	"github.com/civilware/epoch"
	"github.com/deroproject/derohe/globals"
)

func main() {
	globals.Arguments["--testnet"] = true
	globals.Arguments["--simulator"] = true
	globals.InitNetwork()
	// ^ Set up network accordingly for your use, this example is using the simulator with a GetWork server

	// Define the daemon and reward address, then connect to GetWork server
	daemon := "127.0.0.1:20000"
	address := "deto1qyre7td6x9r88y4cavdgpv6k7lvx6j39lfsx420hpvh3ydpcrtxrxqg8v8e3z"
	err := epoch.StartGetWork(address, daemon)
	if err != nil {
		// Handle error
	}
	// The above StartGetWork will connect to port :10100 at daemon 127.0.0.1:20000 by default,
	// a custom GetWork port can be defined calling epoch.SetPort(port) prior to StartGetWork.
	// Once connected, EPOCH will continually update jobs while waiting for calls to attempt or submit hashes.

	// Wait for first job to be ready with a 10 second timeout
	err = epoch.JobIsReady(time.Second * 10)
	if err != nil {
		// Handle error
	}

	// Attempts can be called directly from the package or added to the applications API
	result, err := epoch.AttemptHashes(1000)
	if err != nil {
		// Handle error
	}
	fmt.Printf("EPOCH hash rate: %0.2f H/s\n", result.HashPerSec)

	// Stop EPOCH when done
	epoch.StopGetWork()
}
```

### Examples Using Engram
For applications such as [Engram](https://github.com/DEROFDN/engram) that can handle external connections, EPOCH methods can be seamlessly integrated to to the application and utilized by calling the package's provided API.
```go
import (
	"github.com/civilware/engram/epoch"
	"github.com/deroproject/derohe/walletapi/xswd"
)

func addMethods() {
	xswdServer := xswd.XSWD{}
	for method, function := range epoch.GetHandler() {
		xswdServer.SetCustomMethod(method, function)
	}
}
```

### API
The following sections detail the primary API methods available in the `civilware/epoch` package, including their request and result formats.

#### AttemptEPOCH
Performs the proof of work (POW) and submits the hashes for rewards.

- Request
```json
{
    "jsonrpc": "2.0",
    "id": "1",
    "method": "AttemptEPOCH",
    "params": {
        "hashes": 100
    }
}
```

- Result
```json
{
    "epochHashes": 100,
    "epochSubmitted": 0,
    "epochDuration": 117,
    "epochHashPerSecond": 853.11
}
```

#### SubmitEPOCH
Checks and submits valid precomputed hashes for rewards.

- Request
```json
{
    "jsonrpc": "2.0",
    "id": "1",
    "method": "SubmitEPOCH",
    "params": {
        "jobTemplate": {
            "jobid": "1722895096807.0.notified",
            "blockhashing_blob": "41dc0600000002062bb9d17900000000a12fda3f33403ee25f490fe665a93a3e0000000056790bb6dd9f5bdad5a18d87",
            "difficulty": "1",
            "difficultyuint64": 1,
            "height": 518,
            "prev_hash": "2bb9d1791f7a1865dc8f6f5212a120bd2818b9d8b7345b6b093fda7d3e307425",
            "epochmilli": 0,
            "blocks": 1,
            "miniblocks": 3,
            "rejected": 0,
            "lasterror": "",
            "status": ""
        },
        "powHash": [165, 152, 226, 141, 169, 62, 225, 96, 195, 85, 201, 99, 140, 106, 27, 241, 97, 156, 227, 189, 27, 195, 104, 221, 117, 96, 235, 158, 62, 190, 232, 255],
        "epochWork": [65, 220, 6, 0, 0, 0, 2, 6, 43, 185, 209, 121, 0, 0, 0, 0, 161, 47, 218, 63, 51, 64, 62, 226, 95, 73, 15, 230, 101, 169, 58, 62, 0, 0, 0, 0, 23, 133, 36, 201, 196, 250, 18, 236, 63, 68, 117, 1],
        "epochDifficulty": 1
    }
}
```

- Result
```json
{
    "epochHashes": 1,
    "epochSubmitted": 1,
    "epochDuration": 0,
}
```

##### GetMaxHashesEPOCH
Get the max hash per request currently set by the host application.

- Request
```json
{
    "jsonrpc": "2.0",
    "id": "1",
    "method": "GetMaxHashesEPOCH"
}
```

- Result
```json
{
    "maxHashes": 1000
}
```

##### GetAddressEPOCH
Gets the reward address currently set by the host application.

- Request
```json
{
    "jsonrpc": "2.0",
    "id": "1",
    "method": "GetAddressEPOCH"
}
```

- Result
```json
{
    "epochAddress": "deto1qyre7td6x9r88y4cavdgpv6k7lvx6j39lfsx420hpvh3ydpcrtxrxqg8v8e3z"
}
```

##### GetSessionEPOCH
Gets the current stats for all EPOCH requests that have occurred during a session. The session will include all connected applications in its tally.

- Request
```json
{
    "jsonrpc": "2.0",
    "id": "1",
    "method": "GetSessionEPOCH"
}
```

- Result
```json
{
    "sessionHashes": 1200,
    "sessionMinis": 0
}
```

### Examples Using Tela Applications
TODO: Provide examples for integrating EPOCH with Tela applications.

### Managing User Settings

##### Defining custom values
Custom EPOCH values can be defined prior to calling `StartGetWork` to accommodate various configurations. 
```go
	// Set a custom GetWork port
	epoch.SetPort(9999)
	// Set the max hash amount per request
	epoch.SetMaxHashes(999)
	// Set the max amount of threads EPOCH will use
	epoch.SetMaxThreads(2)
```

##### EPOCH session
EPOCH keeps in memory a tally of successful hashes and submitted miniblocks that occur during a session.
```go
	session, err := epoch.GetSession(time.Second)
	if err != nil {
		// Handle error
	}
	fmt.Printf("Sessions hashes: %d  Session miniblocks: %d\n", session.Hashes, session.MiniBlocks)
```
