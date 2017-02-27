/*
   Mayur's implementation of a Smart Contract for Asset Transfer on Hyperledger Fabric
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

// Structure that represents significant attributes of a User, most importantly their Role
type user struct {
	Id				string	`json:"id"`
	Role			string	`json:"role"`
	AssetBalance	int		`json:"assetBalance"`
}

/* Initialization of global internal constants */
const varJoinedUsersIndex = "_joinedUsersIndex"		//Key name to be used for list of joined users
const varJoinedUsers = "_joinedUsers_"
const initialAssetBalance = 100						//Number of asset units to be allocated to a new joinee
// Labels used to designate valid User Roles within the application:
const roleAdmin = "admin"		//Administrator - permitted to add new users to the network by invoking "join", read-only access to all accounts
const roleUser = "user"			//Standard User - permitted to view & transact on own account only
/* Note: As a convention, system variables stored in the Blockchain begin with an underscore */

// Retrieves the "username" attribute of the chaincode invoker and returns it as a string.
func (t *SimpleChaincode) getUsernameFromEcert(stub shim.ChaincodeStubInterface) (string, error) {
    userName, err := stub.ReadCertAttribute("username");
	if err != nil { panic("ERROR: Failed to get attribute 'username' from eCert of chaincode invoker. Source: getUsernameFromEcert. Details: " + err.Error()) }
	return string(userName), nil
}

// Retrieves User Details for the specified User ID, which must be a previously "join"ed User.
func (t *SimpleChaincode) getUserDetails(stub shim.ChaincodeStubInterface, userId string) (user, error) {
	userDetailsAsBytes, err := stub.GetState(varJoinedUsers + userId)
	if err != nil {
		return user{}, errors.New("ERROR: Failure while getting User Details for User ID '" + userId + "'. Source: getUserDetails. Details: " + err.Error())
	}
	var userDetails user
	errUnmarshalUserDetails := json.Unmarshal(userDetailsAsBytes, &userDetails)
	if errUnmarshalUserDetails != nil {
		panic("ERROR: Failed to unmarshal User Information. Source: getUserDetails. Details: " + errUnmarshalUserDetails.Error())
	}
	return userDetails, nil
}

// Functionality for a new User to join the network. Expects precisely one argument - the unique ID of the joinee
func (t *SimpleChaincode) join(stub shim.ChaincodeStubInterface, userId string, userRole string, args []string) ([]byte, error) {
	const errorHeader = "ERROR: Source: join. "
	if len(args) != 2 {				//Ensure that only the expected number of arguments were passed in
		panic(errorHeader + "Incorrect number of arguments - expecting 2 (Joinee User ID, Joinee User Role).")
	}
	
	// Check that the User performing the operation is either the special "admin" User (to support initial joins) or in an "admin" role
	if userId != "admin" && userRole != roleAdmin {
		panic(errorHeader + "Permission denied - executing User must be 'admin' or assigned an Administrator User Role.")
	}

	// Validate the User ID of the joinee
	joineeId := args[0]
	if strings.Index(joineeId, "_") == 0 {		//Do not accept User IDs beginning with an underscore
		panic(errorHeader + "Source: First input parameter. User ID of new joinee must not begin with an underscore.")
	}
	
	joineeRole := args[1]
	if joineeRole != roleAdmin && joineeRole != roleUser {		//Input Role parameter is invalid
		panic(errorHeader + "Source: Second input parameter. Invalid Role for new joinee.")
	}
	
	// Get the Index of Joined Users
	joinedUsersIndexAsBytes, errGetJoinedUsersIndex := stub.GetState(varJoinedUsersIndex)
	if errGetJoinedUsersIndex != nil {
		panic(errorHeader + "Failed to get Index of Joined Users.")
	}
	var joinedUsersIndex []string
	errUnmarshalJoinedUsersIndex := json.Unmarshal(joinedUsersIndexAsBytes, &joinedUsersIndex)
	if errUnmarshalJoinedUsersIndex != nil {
		panic(errorHeader + "Failed to unmarshal Index of Joined Users.")
	}

	// Validate whether joinee User ID is already present in the Index of Joined Users
	for _, val := range joinedUsersIndex {
		if val == joineeId {
			panic(errorHeader + "A User has previously joined with the same User ID.")
		}
	}

	joineeKey := varJoinedUsers + joineeId				//Create the key name for lookup by concatenating the system variable prefix with the Joinee ID
	joineeDetails := user{Id: joineeId, Role: joineeRole, AssetBalance: initialAssetBalance }			//Create object representing the new joinee
	// Note: Everyone starts with the number of units defined by the 'initialAssetBalance' constant
	joineeDetailsAsBytes, errMarshalJoineeDetails := json.Marshal(joineeDetails)
	if errMarshalJoineeDetails != nil {
		panic(errorHeader + "Failure while marshalling User Details for Joinee ID '" + joineeId + "'. Details: " + errMarshalJoineeDetails.Error())
	}
	errSaveJoineeDetails := stub.PutState(joineeKey, joineeDetailsAsBytes)
	if errSaveJoineeDetails != nil {
		panic(errorHeader + "Failure while storing User Details for Joinee ID '" + joineeId + "'. Details: " + errSaveJoineeDetails.Error())
	}

	// Add joinee User ID to Index of Joined Users and save updated Index to the Blockchain
	var newJoinedUsersIndex []string = append(joinedUsersIndex, joineeId)
	newJoinedUsersIndexAsBytes, errMarshalJoinedUsersIndex := json.Marshal(newJoinedUsersIndex)
	var errMsg string
	if errMarshalJoinedUsersIndex != nil {
		errMsg = errorHeader + "Failure while marshalling Index of Joined Users after adding Joinee ID '" + joineeId + "'. Details: " + errMarshalJoineeDetails.Error()
	}
	errSaveJoinedUsersIndex := stub.PutState(varJoinedUsersIndex, newJoinedUsersIndexAsBytes)
	if errSaveJoinedUsersIndex != nil {
		errMsg = errorHeader + "Failure while adding Joinee ID '" + joineeId + "'. to the Index of Joined Users. Details: " + errSaveJoinedUsersIndex.Error()
	}
	// Roll back addition of User Details if there are any errors while updating the Index of Joined Users
	if errMarshalJoinedUsersIndex != nil || errSaveJoinedUsersIndex != nil {
		errMsg = errMsg + "\nRolling back addition of new User Details... "
		errDeleteJoineeDetails := stub.DelState(joineeKey)
		if errDeleteJoineeDetails == nil {
			errMsg = errMsg + "rollback succeeded for new Joinee ID '" + joineeId + "'."
		} else {
			errMsg = errMsg + "ERROR: ROLLBACK FAILED FOR NEW JOINEE ID '" + joineeId + "'. A SYSTEM ADMINISTRATOR SHOULD IDEALLY PERFORM MANUALLY ROLLBACK BY DELETING STATE FOR THE FOLLOWING KEY FROM WORLD STATE: " + joineeKey + "\nError Details: " + errDeleteJoineeDetails.Error()
		}
		panic(errMsg)
	}

	return nil, nil
}

// Perform Asset Transfer; Users involved must have previously joined the network by invoking the "join" function
// Argument 1: User ID of Sender
// Argument 2: User ID of Receiver
// Argument 3: Asset Quantity to be transferred
func (t *SimpleChaincode) transfer(stub shim.ChaincodeStubInterface, userId string, userRole string, args []string) ([]byte, error) {
	const errorHeader = "ERROR: Source: transfer. "

	if len(args) != 3 {		//Ensure that only the expected number of arguments were passed in
		return nil, errors.New(errorHeader + "Incorrect number of arguments - expecting 3 (Sender ID, Receiver ID, Asset Quantity).")
	}

	// The first two arguments are provided by the system implementation of Invoke()
	senderId := args[0]
	receiverId := args[1]
	// Validate arguments
	if strings.Index(senderId, "_") == 0 || strings.Index(receiverId, "_") == 0 {
		panic(errorHeader + "User IDs of Sender and Receiver must not begin with an underscore.")
	}
	if senderId == receiverId {
		panic(errorHeader + "Sender and Receiver must not be the same User.")
	}
	if userId != senderId {			//Disallow the transaction when the User performing the transfer is the Sender
		panic(errorHeader + "Permission denied - the executing User must match the Sender ID for a successful transfer.")
	}
	assetQuantity, errConvAssetQty := strconv.Atoi(args[2])
	if errConvAssetQty != nil { panic(errorHeader + "Expecting integer value for quantity of Asset to be trasferred.") }
	
	joinedUsersIndexAsBytes, errGetJoinedUsersIndex := stub.GetState(varJoinedUsersIndex)		//Get the Index of Joined Users
	if errGetJoinedUsersIndex != nil {
		panic(errorHeader + "Failed to get Index of Joined Users.")
	}
	var joinedUsersIndex []string
	errUnmarshalJoinedUsersIndex := json.Unmarshal(joinedUsersIndexAsBytes, &joinedUsersIndex)
	if errUnmarshalJoinedUsersIndex != nil {
		panic(errorHeader + "Failed to unmarshal Index of Joined Users.")
	}
	
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
		panic(errorHeader + "Sender is not a member and will have to join the network before attempting this Asset Transfer.")
	}
	if validReceiverId == false {
		panic(errorHeader + "Receiver is not a member and will have to join the network before attempting this Asset Transfer.")
	}
	
	// Retrieve current Asset Balance for Sender from World State
	senderDetails, errGetSenderDetails := t.getUserDetails(stub, senderId)
	if errGetSenderDetails != nil {
		panic(errorHeader + "Failed to get User Information for Sender '" + senderId +"'. Details: " + errGetSenderDetails.Error())
	}
	senderAssetBalance := senderDetails.AssetBalance
	// Validate whether Sender has sufficient Assets to complete the requested transaction 
	newSenderAssetBalance := senderAssetBalance - assetQuantity		//Compute new Sender Balance after transferring specified quantity
	if newSenderAssetBalance < 0 {
		panic(errorHeader + "Sender does not possess sufficient assets to complete the transaction. senderAssetBalance: " + string(senderDetails.AssetBalance))
	}

	// Retrieve current Asset Balance for Receiver from World State
	receiverDetails, errGetReceiverDetails := t.getUserDetails(stub, receiverId)
	if errGetReceiverDetails != nil {
		panic(errorHeader + "Failed to get User Information for Receiver '" + receiverId +"'. Details: " + errGetReceiverDetails.Error())
	}
	receiverAssetBalance := receiverDetails.AssetBalance

	senderDetails.AssetBalance = newSenderAssetBalance
	receiverDetails.AssetBalance = receiverAssetBalance + assetQuantity		//Compute new Receiver Balance after transferring specified quantity

	// Update Sender and Receiver Asset Balances in blockchain World State
	newSenderDetailsAsBytes, errMarshalSenderDetails := json.Marshal(senderDetails)
	if errMarshalSenderDetails != nil {
		panic(errorHeader + "Failure while marshalling updated User Details for Sender '" + senderId + "'. Details: " + errMarshalSenderDetails.Error())
	}
	newReceiverDetailsAsBytes, errMarshalReceiverDetails := json.Marshal(receiverDetails)
	if errMarshalReceiverDetails != nil {
		panic(errorHeader + "Failure while marshalling updated User Details for Receiver ID '" + receiverId + "'. Details: " + errMarshalReceiverDetails.Error())
	}
	errSaveSenderDetails := stub.PutState(varJoinedUsers + senderId, newSenderDetailsAsBytes)
	if errSaveSenderDetails != nil {
		panic(errorHeader + "Transaction failed - unable to update Asset Balance for Sender ID '" + senderId +"'. Details: " + errSaveSenderDetails.Error())
	}
	errSaveRecieverDetails := stub.PutState(varJoinedUsers + receiverId, newReceiverDetailsAsBytes)
	if errSaveRecieverDetails != nil {
		errMsg := errorHeader + "Transaction failed - unable to update Asset Balance for Receiver ID '" + receiverId +"'. Details: " + errSaveRecieverDetails.Error() + "\nAttempting to roll back deduction from Sender account... "
		senderDetails.AssetBalance = senderAssetBalance
		senderDetailsAsBytes, _ := json.Marshal(senderDetails)
		errRollback := stub.PutState(senderId, senderDetailsAsBytes)		//Rollback deduction from sender account
		if errRollback == nil {		//Rollback successful
			panic(errMsg + "successfully rolled back asset deduction of '" + string(assetQuantity) + "' from Sender ID '" + senderId +"'.")
		} else {					//Rollback failed
			panic(errMsg + "CRITICAL ERROR: UNABLE TO ROLL BACK ASSET DEDUCTION OF '" + string(assetQuantity) + "' FROM SENDER ID '" + senderId +"'. WORLD STATE IS INCONSISTENT - TRANSACTION MUST BE MANUALLY REVERSED BY AN ADMINISTRATOR! Correct value: " + strconv.Itoa(senderAssetBalance) + "\nError details: " + errRollback.Error())
		}
	}
	
	fmt.Println("Asset Transfer successful!")
	fmt.Println("New balances: Sender - " + string(senderDetails.AssetBalance) + "; Receiver - " + string(receiverDetails.AssetBalance))
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

	err = stub.PutState(varJoinedUsersIndex, emptyAsBytes) 	//Start with no active users
	if err != nil { return nil, err }
	
	return nil, nil
}

// Invoke is our entry point to invoke various chaincode function
func (t *SimpleChaincode) Invoke(stub shim.ChaincodeStubInterface, function string, args []string) (retBytes []byte, retErr error) {
	const errorHeader = "ERROR: Source: Invoke. "

	defer func() {		//Handle Fatal Errors by translating a call to panic into a returned error 
		retBytes = nil
        fatalErrorMsg := recover().(string)
		retErr = errors.New(errorHeader + "Details: " + fatalErrorMsg)
    }()
	
	fmt.Println("Invoke() is running function '" + function + "'...")

	// Get ID and Details for invoking User
	userId, _ := t.getUsernameFromEcert(stub)
	userDetails, errGetUserDetails := t.getUserDetails(stub, userId)
	if errGetUserDetails != nil {
		return nil, errors.New(errorHeader + "Details: " + errGetUserDetails.Error())
	}

	// Handle different functions
	if function == "init" {					//Used for manual reset
		return t.Init(stub, "init", args)
	} else if function == "join" {			//Used when a new client wishes to join the network
		return t.join(stub, userId, userDetails.Role, args)
	} else if function == "transfer" {		//Used for Asset Transfers
		return t.transfer(stub, userId, userDetails.Role, args)
	}

	fmt.Println("Invoke() did not find function: " + function)					//Log error message
	return nil, errors.New(errorHeader + "Invoke() called with unknown function name: " + function)
}

// Query is our entry point for read operations
func (t *SimpleChaincode) Query(stub shim.ChaincodeStubInterface, function string, args []string) ([]byte, error) {
	fmt.Println("Query() is running function '" + function + "'")
	
	// Handle different functions
	if function == "getassetbalance" {
		if len(args) != 1 {				//Validate the number of arguments
			return nil, errors.New("Incorrect number of arguments - expecting 1 (User ID).")
		}
		userDetails, errGetUserDetails := t.getUserDetails(stub, args[0])	//Get Asset Balance for specified user
		if errGetUserDetails != nil {
			return nil, errors.New("Failed to get Asset Balance for User ID '" + args[0] + "'. Details: " + errGetUserDetails.Error())
		}
		userAssetBalanceAsString := strconv.Itoa(userDetails.AssetBalance)
		fmt.Println("Retrieved Asset Balance: " + userAssetBalanceAsString)
		return []byte(userAssetBalanceAsString), nil;
	} else if function == "getalljoinedusers" {			//Report of all currently joined users and their Asset Balances
		const errorHeader = "ERROR: Source: Query - getalljoinedusers. "
		// Get the Index of Joined Users
		joinedUsersIndexAsBytes, errGetJoinedUsersIndex := stub.GetState(varJoinedUsersIndex)
		if errGetJoinedUsersIndex != nil {
			panic(errorHeader + "Failed to get Index of Joined Users.")
		}
		var joinedUsersIndex []string
		errUnmarshalJoinedUsersIndex := json.Unmarshal(joinedUsersIndexAsBytes, &joinedUsersIndex)
		if errUnmarshalJoinedUsersIndex != nil {
			panic(errorHeader + "Failed to unmarshal Index of Joined Users.")
		}

		fmt.Println("List of Joined Users:")
		if len(joinedUsersIndex) == 0 {
			fmt.Println("No Joined Users found!")
		} else {
			// Validate whether joinee User ID is already present in the Index of Joined Users
			for _, valAsBytes := range joinedUsersIndex {
				fmt.Println("User: " + string(valAsBytes))
			}
		}
		fmt.Println("- X -")
		return joinedUsersIndexAsBytes, nil
	}
	
	fmt.Println("Query() did not find function name: " + function)			//Log error
	return nil, errors.New("Query() received unknown function: " + function)
}