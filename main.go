package main

import (
	"encoding/json"
	"fmt"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/parnurzeal/gorequest"
	"github.com/prometheus/alertmanager/template"
)

type SlackText struct {
	Type string `json:"type"`
	Text string `json:"text"`
}
type SlackBlock struct {
	Type string    `json:"type"`
	Text SlackText `json:"text"`
}

type Attachments struct {
	Blocks []SlackBlock `json:"blocks"`
}

type MinimalSlackPayload struct {
	Blocks      []SlackBlock  `json:"blocks"`
	Text        string        `json:"text"`
	Channel     string        `json:"channel"`
	Icon        string        `json:"icon_emoji"`
	Username    string        `json:"username"`
	Attachments []Attachments `json:"attachments"`
}

var SlackChannel string

func redirectPolicyFunc(req gorequest.Request, via []gorequest.Request) error {
	return fmt.Errorf("Incorrect token (redirection)")
}

// This function sends the slack message
func Send(webhookUrl string, proxy string, payload MinimalSlackPayload) []error {
	request := gorequest.New().Proxy(proxy)
	resp, _, err := request.
		Post(webhookUrl).
		RedirectPolicy(redirectPolicyFunc).
		Send(payload).
		End()

	if err != nil {
		return err
	}
	if resp.StatusCode >= 400 {
		return []error{fmt.Errorf("Error sending msg. Status: %v", resp.Status)}
	}

	return nil
}

func expandAlerts(alert template.Alert) SlackText {
	alertName := alert.Labels["alertname"]
	fmt.Println(alertName)
	slackText := SlackText{}
	slackText.Type = "mrkdwn"
	slackText.Text = "*Alert: * " + alertName
	slackText.Text += "\n*Status:* " + alert.Status
	for k, v := range alert.Annotations {
		slackText.Text += fmt.Sprintf("\n*%s* : %s", k, v)
	}
	slackText.Text += "\n*StartsAt:* _" + alert.StartsAt.String() + "_"
	slackText.Text += "\n*EndsAt:* _" + alert.EndsAt.String() + "_"
	slackText.Text += "\n\n*Details:* "
	for k, v := range alert.Labels {
		slackText.Text += fmt.Sprintf("\n- *_%s_* : `%s`", k, v)
	}
	slackText.Text += "\n\n*GeneratorURL:* " + alert.GeneratorURL
	return slackText
}

// This function generates the SLACK payload
func generateAlertmanagerSlackMessage(data *template.Data, slackChannel string) MinimalSlackPayload {
	minimalSlackPayload := MinimalSlackPayload{}
	minimalSlackPayload.Icon = ":alert:"
	minimalSlackPayload.Channel = slackChannel
	minimalSlackPayload.Username = "AlertBot"
	minimalSlackPayload.Text = "Alerts found"
	slackBlockHeader := SlackBlock{}
	slackBlockHeader.Type = "header"
	slackBlockHeader.Text = SlackText{
		Type: "plain_text",
		Text: fmt.Sprintf("%d Alerts received", len(data.Alerts)),
	}
	minimalSlackPayload.Blocks = append(minimalSlackPayload.Blocks, slackBlockHeader)

	attachmentBlocks := Attachments{}
	for i := 0; i < len(data.Alerts); i++ {
		slackBlock := SlackBlock{}
		slackBlock.Type = "section"
		slackBlock.Text = expandAlerts(data.Alerts[i])
		attachmentBlocks.Blocks = append(attachmentBlocks.Blocks, slackBlock)
	}

	minimalSlackPayload.Attachments = append(minimalSlackPayload.Attachments, attachmentBlocks)
	return minimalSlackPayload
}

// This function handles the SLACK request
func HandleRequest(request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	data := new(template.Data) // creating a template for reponse payload of alertmanager
	ApiResponse := events.APIGatewayProxyResponse{}

	err := json.Unmarshal([]byte(request.Body), &data)
	if err != nil {
		body := "Error: Invalid JSON payload ||| " + fmt.Sprint(err) + " Body Obtained" + "||||" + request.Body
		ApiResponse = events.APIGatewayProxyResponse{Body: body, StatusCode: 500}
	} else {
		minimalSlackPayload := generateAlertmanagerSlackMessage(data, request.QueryStringParameters["channel"])
		sendSlackError := Send("https://hooks.slack.com/services/", "", minimalSlackPayload) // Update the appropriate Slack token here
		if sendSlackError != nil {
			fmt.Println("Error sending Slack message with error ")
			fmt.Print(sendSlackError)
			fmt.Println("######################################")
			ApiResponse = events.APIGatewayProxyResponse{Body: "Error sending Slack message", StatusCode: 502}
		}
		v, erro := json.Marshal(minimalSlackPayload)
		if erro != nil {
			ApiResponse = events.APIGatewayProxyResponse{Body: "Error in generating payload |||" + fmt.Sprint(erro), StatusCode: 200}
		}
		ApiResponse = events.APIGatewayProxyResponse{Body: string(v), StatusCode: 200}

	}
	fmt.Printf(ApiResponse.Body)
	return ApiResponse, nil
}

func main() {
	lambda.Start(HandleRequest)
}
