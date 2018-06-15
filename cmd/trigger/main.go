package main

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/Tinee/hackathon2018/asdasd"
	"github.com/Tinee/hackathon2018/domain"
	"github.com/Tinee/hackathon2018/service"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

// Request from IFTTT
type Request struct {
	TriggerIdentity string `json:"trigger_identity"`
	Triggers        struct {
		From string `json:"hours_start"`
		To   string `json:"hours_stop"`
	} `json:"triggerFields"`
	Limit int `json:"limit"`
}

// Temp empty response
func EmptyResponse() ([]byte, error) {

	return json.Marshal(domain.Response{
		Data: []domain.ResponseDetail{},
	})
}

func ErrorResponse(err error) events.APIGatewayProxyResponse {
	return events.APIGatewayProxyResponse{
		Body:       "{\"errors\": [{\"message\": \"" + err.Error() + "\"}]}",
		StatusCode: 400,
		Headers: map[string]string{
			"content-type": "application/json; charset=utf-8",
		},
	}
}

func BuildResponse(events []domain.ResponseDetail) ([]byte, error) {
	return json.Marshal(domain.Response{
		Data: events,
	})
}

func Handle(e events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	fmt.Println("Starting the application...")
	fmt.Printf("Body %s ", e.Body)

	errr := auth.ValidateIFTTTRequest(e)
	if errr != nil {
		return *errr, nil
	}

	req := Request{}
	req.Limit = -1

	err := json.Unmarshal([]byte(e.Body), &req)
	if err != nil {
		return events.APIGatewayProxyResponse{StatusCode: 400}, nil
	}

	fmt.Printf("Limit is %s", req.Limit)

	fmt.Println("Validating request")
	to := req.Triggers.To
	if to == "" {
		err = errors.New("Missing to")
		return ErrorResponse(err), nil
	}
	from := req.Triggers.From
	if from == "" {
		err = errors.New("Missing from")
		return ErrorResponse(err), nil
	}
	triggerIdentity := req.TriggerIdentity
	if triggerIdentity == "" {
		err = errors.New("Missing triggerIdentity")
		return ErrorResponse(err), nil
	}

	limit := req.Limit

	if limit == 0 {
		fmt.Println("Limit is actually 0 exiting early")
		body, _ := EmptyResponse()

		return events.APIGatewayProxyResponse{Body: string(body), StatusCode: 200, Headers: map[string]string{
			"content-type": "application/json; charset=utf-8",
		}}, nil
	}

	if limit == -1 {
		limit = 0
	}

	fmt.Printf("triggerIdentity: %s\n", triggerIdentity)

	// If there are events in the DB then return those
	existingEvents, err := service.ExistingEvents(triggerIdentity, limit)
	if err != nil {
		fmt.Printf("Error getting the existing events %s\n", err)
		return events.APIGatewayProxyResponse{StatusCode: 500}, nil
	}

	fmt.Printf("existing: %s \n", existingEvents)

	if len(existingEvents) != 0 {
		body, err := BuildResponse(existingEvents)
		if err != nil {
			fmt.Printf("Failed to build response %s\n", err)
			return events.APIGatewayProxyResponse{StatusCode: 500}, nil
		}

		return events.APIGatewayProxyResponse{Body: string(body), StatusCode: 200, Headers: map[string]string{
			"content-type": "application/json; charset=utf-8",
		}}, nil
	}

	// If no existing events and outside window then return an empty array
	inTriggerWindow, err := service.InTriggerWindow(from, to)
	if err != nil {
		fmt.Printf("Error when determining if inside error window %s\n", err)
		return events.APIGatewayProxyResponse{StatusCode: 400}, nil
	}

	if !inTriggerWindow {
		fmt.Println("Exiting early outside of range")
		body, _ := EmptyResponse()

		return events.APIGatewayProxyResponse{Body: string(body), StatusCode: 200, Headers: map[string]string{
			"content-type": "application/json; charset=utf-8",
		}}, nil
	}

	fmt.Println("Saving a new event ")

	// Otherwise lookup the generation
	aggregation, err := service.LookupGreenEnergyPercentage()
	if err != nil {
		fmt.Printf("Error when Looking up green energy percentage %s\n", err)
		return events.APIGatewayProxyResponse{StatusCode: 500}, nil
	}

	isHigher := aggregation > 30.0

	responseDetail, err := service.SaveNewEvent(triggerIdentity, isHigher, aggregation)
	if err != nil {
		fmt.Printf("Error Saving the new event %s\n", err)
		return events.APIGatewayProxyResponse{StatusCode: 500}, nil
	}

	// TODO nicer??
	details := make([]domain.ResponseDetail, 1)
	details[0] = responseDetail

	body, err := BuildResponse(details)
	if err != nil {
		fmt.Printf("Failed to build response %s\n", err)
		return events.APIGatewayProxyResponse{StatusCode: 500}, nil
	}

	return events.APIGatewayProxyResponse{Body: string(body), StatusCode: 200, Headers: map[string]string{
		"content-type": "application/json; charset=utf-8",
	}}, nil
}

func main() {
	lambda.Start(Handle)
}
