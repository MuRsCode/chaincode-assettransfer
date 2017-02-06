/*
   Mayur's implementation of a Smart Contract for Asset Transfer for Hyperledger Fabric
*/

package main

import (
	"errors"
	"fmt"
	"strings"
	"strconv"
	"encoding/json"

	"github.com/hyperledger/fabric/core/chaincode/shim"
)

// Simple Chaincode implementation
type SimpleChaincode struct {
}

// Main - boilerplate code for entry point
func main() {
	err := shim.Start(new(SimpleChaincode))
	if err != nil {
		fmt.Printf("Error starting Simple Chaincode: %s", err)
	}
}

// Global initializations
const joinedUsersStr = "_joinedUsers"		//Key name to be used for list of joined users
const initialAssetBalance = "100"		//Number of asset units to be allocated to a new joinee

// Functionality for a new User to join the network. Expects precisely one argument - the unique ID of the joinee
func (t *SimpleChaincode) join(stub shim.ChaincodeStubInterface, args []string) ([]byte, error) {
	if len(args) != 1 {				//Ensure that the one and only one argument has been passed in
		return nil, errors.New("Incorrect number of arguments for 'join' function - expecting 1.")
	}
	
	// Validate the User ID of the joinee
	joineeId := args[0]
	if strings.Index(joineeId, "_") == 0 {		//Do not accept User IDs beginning with an underscore
		return nil, errors.New("User ID of new joinee must not begin with an underscore.")
	}
	
	// Get the Index of Joined Users
	joinedUsersAsBytes, err := stub.GetState(joinedUsersStr)
	if err != nil {
		return nil, errors.New("Failed to get Index of Joined Users.")
	}
	var joinedUsersIndex []string
	json.Unmarshal(joinedUsersAsBytes, &joinedUsersIndex)
	
	// Validate whether joinee User ID is already present in the Index of Joined Users
	for _, val := range joinedUsersIndex {
		if val == joineeId {
			return nil, errors.New("A User has previously joined with the same User ID.")
		}
	}
	
	// Add joinee User ID to Index of Joined Users and save updated Index to the Blockchain
	var newJoinedUsersIndex []string = append(joinedUsersIndex, joineeId)
	newJoinedUsersIndexAsBytes, _ := json.Marshal(newJoinedUsersIndex)
	err = stub.PutState(joinedUsersStr, newJoinedUsersIndexAsBytes)
	if err != nil {
		return nil, errors.New("Unable to add joinee to the Index of Joined Users.")
	}

	// Add Asset Balance entry to the Blockchain for the joinee User ID (everyone starts with a 100 units)
	err = stub.PutState(joineeId, []byte(initialAssetBalance))
	if err != nil {
		return nil, errors.New("Unable to initialize Asset Balance entry for the joinee User ID. Rolling back addition of User...")
		err = stub.PutState(joineeId, joinedUsersAsBytes)
		if err != nil {
			return nil, errors.New("FATAL ERROR: UNABLE TO AUTOMATICALLY ROLLBACK ADDITION OF NEW USER ID. A SYSTEM ADMINISTRATOR WILL HAVE TO PERFORM THE ROLLBACK MANUALLY TO CORRECT DATABASE. Correct value: " + fmt.Sprint(joinedUsersIndex))
		}
	}
	
	return nil, nil
}

// Perform Asset Transfer; Users involved must have previously joined the network by invoking the "join" function
// Argument 1: User ID of Sender
// Argument 2: User ID of Receiver
// Argument 3: Asset Quantity to be transferred
func (t *SimpleChaincode) transfer(stub shim.ChaincodeStubInterface, args []string) ([]byte, error) {
	if len(args) != 3 {		//Ensure that only the expected number of arguments were passed in
		return nil, errors.New("Incorrect number of arguments for Init invocation - expecting none.")
	}

	senderId := args[0]
	receiverId := args[1]
	assetQuantity, err := strconv.Atoi(args[2])
	if err != nil {
		return nil, errors.New("Expecting integer value for quantity of Asset to be trasferred.")
	}
	
	// Validate User IDs
	if strings.Index(senderId, "_") == 1 || strings.Index(receiverId, "_") == 1 {
		return nil, errors.New("User IDs of Sender and Receiver must not begin with an underscore.")
	}
	if senderId == receiverId {
		return nil, errors.New("Sender and Receiver must not be the same User.")
	}
	joinedUsersAsBytes, err := stub.GetState(joinedUsersStr)		//Get the Index of Joined Users
	if err != nil {
		return nil, errors.New("Failed to get Index of Joined Users.")
	}
	var joinedUsersIndex []string
	json.Unmarshal(joinedUsersAsBytes, &joinedUsersIndex)
	
	// Check that both Sender and Receiver User IDs are already present in the Index of Joined Users
	var validSenderId, validReceiverId bool = false, false
	for _, val := range joinedUsersIndex {
		if val == senderId {
			validSenderId = true
		}
		if val == receiverId {
			validReceiverId = true
		}
	}
	if validSenderId == false {
		return nil, errors.New("Sender is not a member and will have to join the network before attempting this Asset Transfer.")
	}
	if validReceiverId == false {
		return nil, errors.New("Receiver is not a member and will have to join the network before attempting this Asset Transfer.")
	}
	
	// Retrieve current Asset balances for Sender and Receiver from World State
	senderAssetBalanceAsBytes, err := stub.GetState(senderId)
	if err != nil {
		return nil, errors.New("Failed to get Asset Balance for sender '" + senderId +"'. Details: " + err.Error())
	}
	receiverAssetBalanceAsBytes, err := stub.GetState(receiverId)
	if err != nil {
		return nil, errors.New("Failed to get Asset Balance for receiver '" + receiverId +"'. Details: " + err.Error())
	}
	
	senderAssetBalance, err := strconv.Atoi(string(senderAssetBalanceAsBytes))
	if err != nil {
		return nil, errors.New("Transaction failed - failed to retrieve Asset Balance for Sender ID '" + senderId +"'. Error details: " + err.Error())
	}
	newSenderAssetBalance := senderAssetBalance - assetQuantity		//Compute new Sender Balance after transferring specified quantity
	if newSenderAssetBalance < 0 {
		return nil, errors.New("Sender does not possess sufficient assets to complete the transaction. senderAssetBalance: " + string(senderAssetBalanceAsBytes))
	}
	receiverAssetBalance, err := strconv.Atoi(string(receiverAssetBalanceAsBytes))
	if err != nil {
		return nil, errors.New("Transaction failed - failed to retrieve Asset Balance for Receiver ID '" + receiverId +"'. Error details: " + err.Error())
	}
	newReceiverAssetBalance := receiverAssetBalance + assetQuantity		//Compute new Receiver Balance after transferring specified quantity

	// Update results in blockchain World State
	err = stub.PutState(senderId, []byte(strconv.Itoa(newSenderAssetBalance)))
	if err != nil {
		return nil, errors.New("Transaction failed - unable to update Asset Balance for Sender ID '" + senderId +"' . Error details: " + err.Error())
	}
	err = stub.PutState(receiverId, []byte(strconv.Itoa(newReceiverAssetBalance)))
	if err != nil {
		return nil, errors.New("Transaction failed - unable to update Asset Balance for Receiver ID '" + receiverId +"' . Error details: " + err.Error())
		fmt.Println("Attempting to roll back deduction from Sender account...")
		err := stub.PutState(senderId, []byte(strconv.Itoa(senderAssetBalance)))	//Rollback deduction from sender account
		if err != nil {		//Rollback failure
			return nil, errors.New("CRITICAL ERROR: UNABLE TO ROLLBACK ASSET DEDUCTION OF '" + string(assetQuantity) + "' FROM SENDER ID '" + senderId +"' . TRANSACTION MUST BE MANUALLY REVERSED! Error details: " + err.Error())
		}
	}
	
	fmt.Println("Asset Transfer successful!")
	fmt.Println("New balances: Sender - " + string(senderAssetBalance) + "; Receiver - " + string(receiverAssetBalance))

	return nil, nil
}

// Initialize World State
func (t *SimpleChaincode) Init(stub shim.ChaincodeStubInterface, function string, args []string) ([]byte, error) {
	if len(args) != 0 {		//Ensure expected usage of 'Init' without any arguments
		return nil, errors.New("Incorrect number of arguments for Init invocation - expecting none.")
	}
	
	// Initialize Index of Joined Users
	var empty []string
	emptyAsBytes, err := json.Marshal(empty)
	if err != nil {
		return nil, errors.New("Error initializing new Joined User Index. Cannot continue. Details: " + err.Error())
	}

	err = stub.PutState(joinedUsersStr, emptyAsBytes) 	//Start with no active users
	if err != nil {
		return nil, err
	}
	
	return nil, nil
}

// Invoke is our entry point to invoke various chaincode function
func (t *SimpleChaincode) Invoke(stub shim.ChaincodeStubInterface, function string, args []string) ([]byte, error) {
	fmt.Println("Invoke() is running function '" + function + "'")

	// Handle different functions
	if function == "init" {				//Used for manual reset
		return t.Init(stub, "init", args)
	} else if function == "join" {			//Used when a new client wishes to join the network
		return t.join(stub, args)
	} else if function == "transfer" {		//Used for Asset Transfers
		return t.transfer(stub, args)
	}

	fmt.Println("Invoke() did not find function: " + function)					//Log error message
	return nil, errors.New("Invoke() called with unknown function name: " + function)
}

// Query is our entry point for read operations
func (t *SimpleChaincode) Query(stub shim.ChaincodeStubInterface, function string, args []string) ([]byte, error) {
	fmt.Println("Query() is running function '" + function + "'")
	
	// Handle different functions
	if function == "getassetbalance" {
		if len(args) != 1 {				//Validate the number of arguments
			return nil, errors.New("Incorrect number of arguments to Query() - expecting 1.")
		}
		valAsBytes, err := stub.GetState(args[0])	//Get Asset Balance for specified user
		if err != nil {
			return nil, errors.New("Failed to get Asset Balance for User ID '" + args[0] + "'. Details: " + err.Error())
		}
		fmt.Println("Retrieved Asset Balance: " + string(valAsBytes))
		return valAsBytes, nil;
	} else if function == "summary" {			//Report of all currently joined users and their Asset Balances
                // To be implemented
	}

	fmt.Println("Query() did not find function name: " + function)			//Log error
	return nil, errors.New("Query() received unknown function: " + function)
}
