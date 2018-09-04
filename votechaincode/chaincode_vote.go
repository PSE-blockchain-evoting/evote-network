package main

import (
	"errors"
	"fmt"
	"github.com/hyperledger/fabric/core/chaincode/lib/cid"
	"github.com/hyperledger/fabric/core/chaincode/shim"
	pb "github.com/hyperledger/fabric/protos/peer"
	"encoding/json"
	"strconv"
	"time"
)

type VoteChaincode struct {
}

func (t *VoteChaincode) Init(stub shim.ChaincodeStubInterface) pb.Response {
	fmt.Println("Chaincode Init")
	return shim.Success(nil)
}

func (t *VoteChaincode) Invoke(stub shim.ChaincodeStubInterface) pb.Response {
	fmt.Println("Vote Invoke")
	function, args := stub.GetFunctionAndParameters()
	if function == "allVotesQuery" {
		// Retrieve all submitted votes.
		return t.allVotesQuery(stub, args)
	} else if function == "electionStatusQuery" {
		// Check if election has ended.
		return t.electionStatusQuery(stub, args)
	} else if function == "ownVoteQuery" {
		// Retrieve the own vote.
		return t.ownVoteQuery(stub, args)
	} else if function == "electionDataQuery" {
		// Retrieve metadata about the election.
		return t.electionDataQuery(stub, args)
	} else if function == "destructionInvokation" {
		// Clears the current election.
		return t.destructionInvokation(stub, args)
	} else if function == "initializationInvokation" {
		// Initializes the election with metadata.
		return t.initializationInvokation(stub, args)
	} else if function == "voteInvokation" {
		// Submits vote to chaincode.
		return t.voteInvokation(stub, args)
	} else if function == "initStatusQuery" {
		//Check if an election is initialized
		return t.initStatusQuery(stub, args)
	}

	return shim.Error("Invalid invoke function name. Expecting \"vote\" \"query\"")
}

func (t *VoteChaincode) allVotesQuery(stub shim.ChaincodeStubInterface, args []string) pb.Response {
	var resultSlice []string = []string{}

	stateIterator, err := stub.GetStateByRange("v", "w")
	if err != nil {
		return shim.Error("Failed to get StateIterator")
	}
	defer stateIterator.Close()

	for stateIterator.HasNext() {
		queryResponse, err := stateIterator.Next()
		if err != nil {
			return shim.Error("StateIterator failed to retrieve next Element")
		}
		resultSlice = append(resultSlice, string(queryResponse.Value))
	}

	returnJson, err := json.Marshal(resultSlice)
	if err != nil {
		return shim.Error("Failed to generate Json")
	}
	return shim.Success(returnJson)
}

func electionStartedEndedCheck(stub shim.ChaincodeStubInterface) (started,ended bool,err error) {
	var initMap map[string]*json.RawMessage
	var endConditionMap map[string]*json.RawMessage

	//Retrieve Metadata from init Block
	stateBytes, err := stub.GetState("init")
	if err != nil {
		return false,false,errors.New("Failed to get state")
	}
	if stateBytes == nil {
		return false,false, errors.New("Init not set")
	}

	err = json.Unmarshal(stateBytes, &initMap)
	if err != nil {
		return false, false, errors.New("Json couldn't be parsed, maybe the initialization was done incorrectly.")
	}

	err = json.Unmarshal([]byte(*initMap["endCondition"]), &endConditionMap)
	if err != nil {
		return false,false, errors.New("Json of endCondition couldn't be parsed, maybe the initialization was done incorrectly.")
	}

	//Check Time for all electionEndTypes
	endTimeInt, err := strconv.ParseInt(string(*initMap["endDate"]), 10, 64)
	if err != nil {
		return false,false, errors.New("endDate couldn't be parsed correctly, maybe the initialization was done incorrectly.")
	}
	startTimeInt, err := strconv.ParseInt(string(*initMap["startDate"]), 10, 64)
	if err != nil {
		return false,false, errors.New("startDate couldn't be parsed correctly, maybe the initialization was done incorrectly.")
	}

	startTime := time.Unix(startTimeInt, 0)
	endTime := time.Unix(endTimeInt, 0)
	now := time.Now()
	startedBool := now.After(startTime)
	
	debugTimes := "start: "
	debugTimes += strconv.FormatInt(startTime.Unix(),10)
	debugTimes += "; now: "
	debugTimes += strconv.FormatInt(now.Unix(),10)
	debugTimes += "; end: "
	debugTimes += strconv.FormatInt(endTime.Unix(),10)
	fmt.Println(debugTimes)

	if now.After(endTime) {
		return startedBool,true, nil
	}
	//Check for VoterPercentileCondition
	if string(*endConditionMap["type"]) == "\"VoterPercentileCondition\"" {
		neededPercentage, err := strconv.Atoi(string(*endConditionMap["percentage"]))
		if err != nil {
			return false,false, errors.New("Failed to parse percentage for VoterPercentileCondition")
		}
		stateIterator, err := stub.GetStateByRange("v", "w")
		if err != nil {
			return false,false, errors.New("Failed to get StateIterator")
		}
		defer stateIterator.Close()

		numVotes := 0
		for stateIterator.HasNext() {
			numVotes++
			stateIterator.Next()
		}

		numAllVoters, err := strconv.Atoi(string(*initMap["voterCount"]))
		if err != nil {
			return false,false, errors.New("Failed to parse voterCount")
		}

		actualPercentage := int((float64(numVotes) / float64(numAllVoters))*100.0)
		if actualPercentage >= neededPercentage {
			return startedBool,true,nil
		}
	} else if string(*endConditionMap["type"]) == "\"CandidatePercentileCondition\"" {
		neededPercentage, err := strconv.Atoi(string(*endConditionMap["percentage"]))
		if err != nil {
			return false,false, errors.New("Failed to parse percentage for VoterPercentileCondition")
		}

		stateIterator, err := stub.GetStateByRange("v", "w")
		if err != nil {
			return false,false, errors.New("Failed to get StateIterator")
		}
		defer stateIterator.Close()

		var uniqueVotes []string
		var numVotes []int
		for stateIterator.HasNext() {
			queryResponse, err := stateIterator.Next()
			if err != nil {
				return false,false, errors.New("StateIterator failed to retrieve next Element")
			}
			voteString := string(queryResponse.Value)
			uniquePos := posOf(voteString,uniqueVotes)
			if uniquePos == -1{
				uniqueVotes = append(uniqueVotes, voteString)
				numVotes = append(numVotes,1)
			} else {
				numVotes[uniquePos]+=1
			}
		}

		numAllVoters, err := strconv.Atoi(string(*initMap["voterCount"]))
		if err != nil {
			return false,false, errors.New("Failed to parse voterCount")
		}

		for _, num := range numVotes {
			actualPercentage := int((float64(num) / float64(numAllVoters))*100.0)
			if actualPercentage >= neededPercentage {
				return startedBool,true,nil
			}
		}
	}

	return startedBool,false,nil
}


func (t *VoteChaincode) electionStatusQuery(stub shim.ChaincodeStubInterface, args []string) pb.Response {
	_, ended, err := electionStartedEndedCheck(stub)
	if err != nil {
		return shim.Error(err.Error())
	}
	if ended {
		return shim.Success([]byte("ended"))
	} else {
		return shim.Success([]byte("running"))
	}
}

func (t *VoteChaincode) ownVoteQuery(stub shim.ChaincodeStubInterface, args []string) pb.Response {
	creatorID, err := cid.GetID(stub)
	if err != nil {
		return shim.Error("Couldn't read ID from stub.")
	}
	key := "vote_" + creatorID
	stateBytes, err := stub.GetState(key)
	if err != nil {
		return shim.Success(nil)
	}

	return shim.Success(stateBytes)
}

// Query Election Metadata on ledger.
func (t *VoteChaincode) electionDataQuery(stub shim.ChaincodeStubInterface, args []string) pb.Response {
	stateBytes, err := stub.GetState("init")
	if err != nil {
		return shim.Error("Failed to get state")
	}
	if stateBytes == nil {
		return shim.Error("Election not initialized")
	}

	fmt.Printf("Responding with ElectionData: " + string(stateBytes))
	return shim.Success(stateBytes)
}

func (t *VoteChaincode) destructionInvokation(stub shim.ChaincodeStubInterface, args []string) pb.Response {
	err := cid.AssertAttributeValue(stub,"admin","true")
	if err != nil {
		return shim.Error("User isn't admin")
	}
	fmt.Println("RESTART")
	return shim.Success(nil)
}

// Write ElectionData on ledger with 'init' key.
func (t *VoteChaincode) initializationInvokation(stub shim.ChaincodeStubInterface, args []string) pb.Response {
	var initMap map[string]*json.RawMessage
	var endConditionMap map[string]*json.RawMessage

	err := cid.AssertAttributeValue(stub,"admin","true")
	if err != nil {
		return shim.Error("User isn't admin")
	}

	if len(args) != 1 {
		return shim.Error("Incorrect number of arguments. Expecting a single JSON string representing ElectionData")
	}

	var initJson = args[0]

	stateBytes, err := stub.GetState("init")
	if err != nil {
		return shim.Error("Failed to get state")
	}
	if stateBytes != nil {
		return shim.Error("Init set already")
	}

	err = json.Unmarshal([]byte(initJson), &initMap)
	if err != nil {
		return shim.Error("Json couldn't be parsed, maybe the initialization was done incorrectly.")
	}
	err = json.Unmarshal([]byte(*initMap["endCondition"]), &endConditionMap)
	if err != nil {
		return shim.Error("endCondition couldn't be parsed, maybe the initialization was done incorrectly.")
	}

	_, err = strconv.ParseInt(string(*initMap["endDate"]), 10, 64)
	if err != nil {
		return shim.Error("The given Time couldn't be parsed: " + string(*initMap["endDate"]))
	}

	if endConditionMap["type"] == nil {
		return shim.Error("Endcondition Type couldn't be parsed, maybe the initialization was done incorrectly.")
	}
	if string(*endConditionMap["type"]) != "\"TimeOnlyCondition\"" {
		if endConditionMap["percentage"] == nil {
			return shim.Error("Percentage couldn't be parsed, maybe the initialization was done incorrectly.")
		}
	}

	err = stub.PutState("init", []byte(initJson))
	if err != nil {
		return shim.Error(err.Error())
	}

	fmt.Println("Init written to Ledger:")
	fmt.Println(initJson)
	return shim.Success(nil)
}

func (t *VoteChaincode) voteInvokation(stub shim.ChaincodeStubInterface, args []string) pb.Response {
	started, ended, err := electionStartedEndedCheck(stub)
	if err != nil {
		return shim.Error(err.Error())
	}
	if !started || ended {
		runningDebug := "Election started: "
		runningDebug += strconv.FormatBool(started)
		runningDebug += "; Election ended: "
		runningDebug += strconv.FormatBool(ended)
		fmt.Println(runningDebug)
		return shim.Error("Election isn't running")
	}

	err = cid.AssertAttributeValue(stub,"admin","true")
	if err == nil {
		return shim.Error("User is admin therefore is not allowed to vote")
	}
	if len(args) != 1 {
		return shim.Error("Incorrect number of arguments. Expecting a single JSON string representing a Vote")
	}
	var voteJson = args[0]

	creatorID, err := cid.GetID(stub)
	if err != nil {
		return shim.Error("Couldn't read ID from stub.")
	}
	key := "vote_" + creatorID
	stateBytes, err := stub.GetState(key)
	if err != nil {
		return shim.Error("Failed to get state")
	}
	if stateBytes != nil {
		return shim.Error("User already voted once")
	}

	err = stub.PutState(key, []byte(voteJson))
	if err != nil {
		return shim.Error(err.Error())
	}

	return shim.Success(nil)
}

func (t *VoteChaincode) initStatusQuery(stub shim.ChaincodeStubInterface, args []string) pb.Response {

	stateBytes, err := stub.GetState("init")
	if err != nil {
		return shim.Success([]byte("true"))
	}
	if stateBytes != nil {
		return shim.Success([]byte("true"))
	}
	return shim.Success([]byte("false"))
}

//Utility function
func posOf(search string,array []string) int {
	for p, v := range array {
		if v == search {
			return p
		}
	}
	return -1
}

func main() {
	err := shim.Start(new(VoteChaincode))
	if err != nil {
		fmt.Printf("Error starting Vote chaincode: %s", err)
	}
}