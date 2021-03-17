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
	TopicArn   string = "arn:aws:sns:us-east-1:12334567:Covid-vaccine" //change default
	AWS_region string = "us-east-1"
	State      string = "IA"
)

var fnvHash hash.Hash32 = fnv.New32a()

func main() {
	lambda.Start(HandleRequest) //*IMPORTANT* comment/remove for local testing
	//getVaccine() //*IMPORTANT*  uncomment for local testing
}

//HandleRequest : lambda execution
func HandleRequest(ctx context.Context) (string, error) {
	str, err := getVaccine()
	return str, err
}

func getVaccine() (string, error) {

	STATE := getEnvState()
	fmt.Printf("STATE: %v\n", getEnvState())

	timeout := time.Duration(5 * time.Second)
	client := http.Client{
		Timeout: timeout,
	}

	request, err := http.NewRequest("GET", "https://www.vaccinespotter.org/api/v0/states/IA.json", nil)
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

	//log.Println(string(body))

	bytes := body
	var res RESPBODY
	json.Unmarshal(bytes, &res)

	//fmt.Println(res.Features)
	var available properties

	for _, val := range res.Features {

		// fmt.Printf("ID: %v\n", val.Properties.ID)
		// fmt.Printf("URL: %s\n", val.Properties.URL)
		// fmt.Printf("City: %s\n", val.Properties.City)
		// fmt.Printf("Name: %s\n", val.Properties.Name)
		// fmt.Printf("State: %s\n", val.Properties.State)
		// fmt.Printf("Address: %s\n", val.Properties.Address)
		// fmt.Printf("Provider: %s\n", val.Properties.Provider)
		// fmt.Printf("TimeZone: %s\n", val.Properties.TimeZone)
		// fmt.Printf("PostalCode: %s\n", val.Properties.PostalCode)
		//fmt.Printf("Appointments: %s\n", val.Properties.Appointments)
		// fmt.Printf("ProviderBrand: %s\n", val.Properties.ProviderBrand)
		// fmt.Printf("CarriesVaccine: %t\n", val.Properties.CarriesVaccine)
		// fmt.Printf("Location: %s\n", val.Properties.ProviderBrandName)
		// fmt.Printf("ProviderBrandName: %s\n", val.Properties.ProviderLocationID)
		// fmt.Printf("AppointmentsAvailable: %t\n", val.Properties.AppointmentsAvailable)
		// fmt.Printf("AppointmentsLastFetched: %s\n", val.Properties.AppointmentsLastFetched)
		// fmt.Println("=============")

		if val.Properties.AppointmentsAvailable && val.Properties.State == STATE || len(val.Properties.Appointments) > 0 {
			available = append(available, val)
		}
	}

	if len(available) > 0 {
		// for _, val := range available {
		// 	fmt.Printf("Address: %s\n", val.Properties.Address)
		// 	fmt.Printf("Provider: %s\n", val.Properties.Provider)
		// 	fmt.Printf("TimeZone: %s\n", val.Properties.TimeZone)
		// 	fmt.Printf("PostalCode: %s\n", val.Properties.PostalCode)
		// 	fmt.Printf("Appointments: %s\n", val.Properties.Appointments)
		// 	fmt.Printf("ProviderBrand: %s\n", val.Properties.ProviderBrand)
		// 	fmt.Printf("CarriesVaccine: %t\n", val.Properties.CarriesVaccine)
		// 	fmt.Printf("Location: %s\n", val.Properties.ProviderBrandName)
		// 	fmt.Printf("ProviderBrandName: %s\n", val.Properties.ProviderLocationID)
		// 	fmt.Printf("AppointmentsAvailable: %t\n", val.Properties.AppointmentsAvailable)
		// 	fmt.Printf("AppointmentsLastFetched: %s\n", val.Properties.AppointmentsLastFetched)
		// 	fmt.Println()
		// 	fmt.Println("=============")
		// }
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

func composeMessage(available properties) string {
	//compose message
	var resultStr strings.Builder
	resultStr.WriteString("Vaccination available at:\n")
	resultStr.WriteString("Local pharmacies + Hyvee\n")
	for _, val := range available {
		location := fmt.Sprintf("Location: %s\n", val.Properties.Name)
		resultStr.WriteString(location)
		url := fmt.Sprintf("URL: %s\n", val.Properties.URL)
		resultStr.WriteString(url)
		address := fmt.Sprintf("Address: %s, %s, %s, %s\n", val.Properties.Address, val.Properties.City, val.Properties.State, val.Properties.PostalCode)
		resultStr.WriteString(address)
		// carriesVaccine := fmt.Sprintf("Carries Vaccine: %t\n", val.Properties.CarriesVaccine)
		// resultStr.WriteString(carriesVaccine)
		appointmentsAvailable := fmt.Sprintf("Appointments Available: %t\n", val.Properties.AppointmentsAvailable)
		resultStr.WriteString(appointmentsAvailable)
		resultStr.WriteString("- - -\n")
	}

	return resultStr.String()
}

func sendMessage(message string) (string, error) {
	fmt.Println("Sending sns message")
	sess, err := session.NewSession(&aws.Config{
		Region: aws.String(AWS_region),
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

	result, err := client.Publish(input)
	if err != nil {
		fmt.Println("Publish error:", err)
		return "ERROR publishing...", err
	}

	fmt.Println(result)
	output := fmt.Sprintf("%s", result)
	return output, nil
}

func updateDatabase(hash string) bool {

	sess, errSession := session.NewSession(&aws.Config{
		Region: aws.String(AWS_region),
	})

	if errSession != nil {
		fmt.Println("NewSession error:", errSession)
		return false
	}
	// Create DynamoDB client
	svc := dynamodb.New(sess)

	// Update item in table Covid
	tableName := "Covid"
	source := "covid-all-location"
	id := "2019"

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

func getHash(s string) string {
	fnvHash.Write([]byte(s))
	defer fnvHash.Reset()

	return fmt.Sprintf("%x", fnvHash.Sum(nil))
}

func getEnvState() string {
	v := os.Getenv("STATE")
	if v == "" {
		return State
	}
	return v
}

func getEnvTopic() string {
	v := os.Getenv("TOPIC_ARN")
	if v == "" {
		return TopicArn
	}
	return v
}

type Covid struct {
	ID          int
	Source      string
	Fingerprint string
}

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
