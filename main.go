package main

import (
	"context"
	"encoding/json"
	"fmt"
	"hash"
	"hash/fnv"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/aws/aws-sdk-go/service/sns"
)

var (
	TopicArn     string = "arn:aws:sns:us-east-1:12334567:Covid-vaccine" //change default
	AWSRegion    string = "us-east-1"
	State        string = "IA"
	Table        string = "xyz"
	ID           string = "2019"
	Source       string = "covid-vaccine-notifier"
	RangeA       string = "00000"
	RangeB       string = "99000"
	MuteProvider string = "unknown"
	EndPoint     string = "https://www.vaccinespotter.org/api/v0/states/%s.json"
)

var fnvHash hash.Hash32 = fnv.New32a()

func main() {
	lambda.Start(HandleRequest) //*IMPORTANT* comment/remove for local testing
	//getVaccine() //*IMPORTANT*  uncomment for local testing
}

/* HandleRequest : lambda execution */
func HandleRequest(ctx context.Context) (string, error) {
	str, err := getVaccine()
	return str, err
}

/* Get vaccination information */
func getVaccine() (string, error) {

	STATE := getEnvState()
	RANGE_A := getEnvZipRangeA()
	RANGE_B := getEnvZipRangeB()
	MUTE := getEnvMuteProvider()
	fmt.Printf("STATE: %v\n", STATE)
	fmt.Printf("RANGE_A: %v\n", RANGE_A)
	fmt.Printf("RANGE_B: %v\n", RANGE_B)

	//get slice of muted provider
	mutedList := strings.Split(MUTE, ",")
	fmt.Printf("MUTE: %v\n", mutedList)
	//put muted list in map
	mutedHash := make(map[string]bool)
	for _, val := range mutedList {
		mutedHash[val] = true
	}

	timeout := time.Duration(5 * time.Second)
	client := http.Client{
		Timeout: timeout,
	}
	endpoint := fmt.Sprintf(EndPoint, STATE)
	request, err := http.NewRequest("GET", endpoint, nil)
	request.Header.Set("Content-type", "application/json")

	if err != nil {
		fmt.Println(err)
		return "", err
	}

	resp, err := client.Do(request)

	if err != nil {
		fmt.Println(err)
		return "", err
	}

	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println(err)
		return "", err
	}

	bytes := body
	var res RESPBODY
	json.Unmarshal(bytes, &res)
	var available properties

	for _, val := range res.Features {
		mutedProvider, _ := mutedHash[val.Properties.ProviderBrand]

		if val.Properties.AppointmentsAvailable && val.Properties.State == STATE && (convertToInt(val.Properties.PostalCode) >= convertToInt(RANGE_A) && convertToInt(val.Properties.PostalCode) <= convertToInt(RANGE_B)) && !mutedProvider {
			available = append(available, val)
		}
	}

	//if appoinments are available
	if len(available) > 0 {
		message := composeMessage(available)
		fmt.Println(message)
		hash := getHash(message)
		fmt.Println(hash)
		if updateDatabase(hash) {
			return sendMessage(message)
		}
	}
	return "Nothing to do", nil
}

/* Compose message */
func composeMessage(available properties) string {
	var resultStr strings.Builder
	resultStr.WriteString("Vaccination appointments available at:\n")
	resultStr.WriteString("Local pharmacies\n")
	for _, val := range available {
		location := fmt.Sprintf("Location: %s\n", val.Properties.Name)
		resultStr.WriteString(location)
		url := fmt.Sprintf("URL: %s\n", val.Properties.URL)
		resultStr.WriteString(url)
		address := fmt.Sprintf("Address: %s, %s, %s, %s\n", val.Properties.Address, val.Properties.City, val.Properties.State, val.Properties.PostalCode)
		resultStr.WriteString(address)
		appointmentsAvailable := fmt.Sprintf("Appointments Available: %t\n", val.Properties.AppointmentsAvailable)
		resultStr.WriteString(appointmentsAvailable)
		resultStr.WriteString("- - -\n")
	}

	return resultStr.String()
}

/* Notify SNS Topic */
func sendMessage(message string) (string, error) {
	fmt.Println("Sending sns message")
	sess, err := session.NewSession(&aws.Config{
		Region: aws.String(AWSRegion),
	})

	if err != nil {
		fmt.Println("NewSession error:", err)
		return "Unable to create session", err
	}

	client := sns.New(sess)
	input := &sns.PublishInput{
		Message:  aws.String(message),
		TopicArn: aws.String(getEnvTopic()),
	}
	//return "output", nil //this is used for debugging - uncommenting this will prevent message from being sent
	result, err := client.Publish(input)
	if err != nil {
		fmt.Println("Publish error:", err)
		return "ERROR publishing...", err
	}

	fmt.Println(result)
	output := fmt.Sprintf("%s", result)
	return output, nil
}

/* Peek and update database function */
func updateDatabase(hash string) bool {
	sess, errSession := session.NewSession(&aws.Config{
		Region: aws.String(AWSRegion),
	})

	if errSession != nil {
		fmt.Println("NewSession error:", errSession)
		return false
	}
	// Create DynamoDB client
	svc := dynamodb.New(sess)
	tableName := getEnvTable()
	source := getEnvSource()
	id := getEnvTableID()

	result, err := svc.GetItem(&dynamodb.GetItemInput{
		TableName: aws.String(tableName),
		Key: map[string]*dynamodb.AttributeValue{
			"Source": {
				S: aws.String(source),
			},
			"ID": {
				N: aws.String(id),
			},
		},
	})

	if err != nil {
		fmt.Println(err.Error())
		return false
	}

	if result.Item == nil {
		fmt.Println("Could not find item..continue")
	}

	item := Covid{}
	err = dynamodbattribute.UnmarshalMap(result.Item, &item)

	if err != nil {
		panic(fmt.Sprintf("Failed to unmarshal Record, %v", err))
	}

	if result != nil {
		fmt.Println("Found item:")
		fmt.Println("Source:  ", item.Source)
		fmt.Println("fingerprint: ", item.Fingerprint)
		fmt.Println("ID:", item.ID)
		fmt.Println("hash: ", hash)
	}

	if item.Fingerprint == hash {
		fmt.Println("No need to update since nothing changed")
		return false
	} else {
		input := &dynamodb.UpdateItemInput{
			ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
				":f": {
					S: aws.String(hash),
				},
			},
			TableName: aws.String(tableName),
			Key: map[string]*dynamodb.AttributeValue{
				"ID": {
					N: aws.String(id),
				},
				"Source": {
					S: aws.String(source),
				},
			},
			ReturnValues:     aws.String("UPDATED_NEW"),
			UpdateExpression: aws.String("set Fingerprint = :f"),
		}

		_, err2 := svc.UpdateItem(input)
		if err2 != nil {
			fmt.Println(err2.Error())
			return false
		}

		fmt.Println("Successfully updated dynamo")
		return true
	}
}

/* provide Hash for fingerprint */
func getHash(s string) string {
	fnvHash.Write([]byte(s))
	defer fnvHash.Reset()
	return fmt.Sprintf("%x", fnvHash.Sum(nil))
}

/* get ENV for AWS Region */
func getEnvRegion() string {
	v := os.Getenv("AWS_REGION")
	if v == "" {
		return AWSRegion
	}
	return v
}

/* get ENV for STATE NAME only 2 letters allowed */
func getEnvState() string {
	v := os.Getenv("STATE")
	if v == "" {
		return State
	}
	return v
}

/* get ENV for TOPIC ARN */
func getEnvTopic() string {
	v := os.Getenv("TOPIC_ARN")
	if v == "" {
		return TopicArn
	}
	return v
}

/* get ENV for DB TABLE NAME */
func getEnvTable() string {
	v := os.Getenv("TABLE_NAME")
	if v == "" {
		return Table
	}
	return v
}

/* get ENV for DB TABLEID */
func getEnvTableID() string {
	v := os.Getenv("TABLE_ID")
	if v == "" {
		return ID
	}
	return v
}

/* get ENV for DB Source */
func getEnvSource() string {
	v := os.Getenv("SOURCE")
	if v == "" {
		return Source
	}
	return v
}

/* get ENV for zipcode range START */
func getEnvZipRangeA() string {
	v := os.Getenv("RANGE_A")
	if v == "" {
		return RangeA
	}
	return v
}

/* get ENV for zipcode range END */
func getEnvZipRangeB() string {
	v := os.Getenv("RANGE_B")
	if v == "" {
		return RangeB
	}
	return v
}

/* get ENV for muted pharmacies */
func getEnvMuteProvider() string {
	v := os.Getenv("MUTE")
	if v == "" {
		return MuteProvider
	}
	return v
}

/* convert string to int */
func convertToInt(str string) int {
	i, err := strconv.Atoi(str)
	if err != nil {
		return 0
	}
	return i
}

/* Covid: Database structure */
type Covid struct {
	ID          int
	Source      string
	Fingerprint string
}

/* properties: structure to get vaccination site info*/
type properties []struct {
	Type     string `json:"type"`
	Geometry struct {
		Type        string    `json:"type"`
		Coordinates []float64 `json:"coordinates"`
	} `json:"geometry"`
	Properties struct {
		ID                      int           `json:"id"`
		URL                     string        `json:"url"`
		City                    string        `json:"city"`
		Name                    string        `json:"name"`
		State                   string        `json:"state"`
		Address                 string        `json:"address"`
		Provider                string        `json:"provider"`
		TimeZone                string        `json:"time_zone"`
		PostalCode              string        `json:"postal_code"`
		Appointments            []interface{} `json:"appointments"`
		ProviderBrand           string        `json:"provider_brand"`
		CarriesVaccine          bool          `json:"carries_vaccine"`
		ProviderBrandName       string        `json:"provider_brand_name"`
		ProviderLocationID      string        `json:"provider_location_id"`
		AppointmentsAvailable   bool          `json:"appointments_available"`
		AppointmentsLastFetched time.Time     `json:"appointments_last_fetched"`
	} `json:"properties"`
}

/* RESPBODY: structure Response body from API call*/
type RESPBODY struct {
	Type     string `json:"type"`
	Features []struct {
		Type     string `json:"type"`
		Geometry struct {
			Type        string    `json:"type"`
			Coordinates []float64 `json:"coordinates"`
		} `json:"geometry"`
		Properties struct {
			ID                      int           `json:"id"`
			URL                     string        `json:"url"`
			City                    string        `json:"city"`
			Name                    string        `json:"name"`
			State                   string        `json:"state"`
			Address                 string        `json:"address"`
			Provider                string        `json:"provider"`
			TimeZone                string        `json:"time_zone"`
			PostalCode              string        `json:"postal_code"`
			Appointments            []interface{} `json:"appointments"`
			ProviderBrand           string        `json:"provider_brand"`
			CarriesVaccine          bool          `json:"carries_vaccine"`
			ProviderBrandName       string        `json:"provider_brand_name"`
			ProviderLocationID      string        `json:"provider_location_id"`
			AppointmentsAvailable   bool          `json:"appointments_available"`
			AppointmentsLastFetched time.Time     `json:"appointments_last_fetched"`
		} `json:"properties"`
	} `json:"features"`
	Metadata struct {
		Code        string `json:"code"`
		Name        string `json:"name"`
		StoreCount  int    `json:"store_count"`
		BoundingBox struct {
			Type        string        `json:"type"`
			Coordinates [][][]float64 `json:"coordinates"`
		} `json:"bounding_box"`
		ProviderBrands []struct {
			ID                      int       `json:"id"`
			Key                     string    `json:"key"`
			URL                     string    `json:"url"`
			Name                    string    `json:"name"`
			Status                  string    `json:"status"`
			ProviderID              string    `json:"provider_id"`
			LocationCount           int       `json:"location_count"`
			AppointmentsLastFetched time.Time `json:"appointments_last_fetched"`
		} `json:"provider_brands"`
		ProviderBrandCount      int       `json:"provider_brand_count"`
		AppointmentsLastFetched time.Time `json:"appointments_last_fetched"`
	} `json:"metadata"`
}
