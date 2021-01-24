package main

import (
	"encoding/json"
	"fmt"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/s3"
	"log"
	"net"

	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"context"
	"github.com/shadowscatcher/shodan"
	"github.com/shadowscatcher/shodan/search"
	"github.com/slack-go/slack"
	"os"
)

type Event struct {
	Region string `json:"region"`
	Bucket string `json:"bucket`
}

type Result struct {
	Message string `json:"RESULT:"`
}

type Record struct {
	Timestamp string
	Name string
	Value string
	Type string
}

func main() {
	lambda.Start(Handler)
	//Debug()
}

func Debug() {
	publicIp := "52.89.70.92"
	findTargetsOnShodan(publicIp)
}

func Handler(event Event) (Result, error){
	region := event.Region
	publicIp, allocationId := allocateAddress(region)

	log.Printf("Checking %s from allocation ID %s\n", publicIp, allocationId)

	if findTargetsOnShodan(publicIp) || findTargetsOnS3(publicIp) {
		return Result{Message: fmt.Sprintf("found target on %s", publicIp)}, nil
	} else {
		releaseAddress(region, publicIp, allocationId)
		return Result{Message: fmt.Sprintf("no matches on %s", publicIp)}, nil
	}

	return Result{Message: fmt.Sprintf("processed %s", publicIp)}, nil
}

func allocateAddress(region string)  (string, string) {
	sess, err := session.NewSession(&aws.Config{
		Region: aws.String(region)},
	)

	svc := ec2.New(sess)

	allocRes, err := svc.AllocateAddress(&ec2.AllocateAddressInput{
		Domain: aws.String("vpc"),
	})

	if err != nil {
		exitErrorf("Error allocating EIP, %v", err)
	}

	return *allocRes.PublicIp, *allocRes.AllocationId
}

func releaseAddress(region string, publicIp string, allocationId string) {
	if len(allocationId) > 0 {
		sess, err := session.NewSession(&aws.Config{
			Region: aws.String(region)},
		)

		svc := ec2.New(sess)

		_, err = svc.ReleaseAddress(&ec2.ReleaseAddressInput{
			AllocationId: aws.String(allocationId),
		})
		if err != nil {
			if aerr, ok := err.(awserr.Error); ok && aerr.Code() == "InvalidAllocationID.NotFound" {
				exitErrorf("Allocation ID %s does not exist", allocationId)
			}
			exitErrorf("Unable to release IP address for allocation %s, %v", allocationId, err)
		}

		log.Printf("Released %s from allocation ID %s\n", publicIp, allocationId)
	} else {
		log.Printf("No allocation ID to release for %s\n", publicIp)
	}
}

func findTargetsOnS3(publicIp string) bool {
	firstOctet := strings.Split(publicIp, ".")[0]
	key := "rdns/rdns." + firstOctet + ".0.0.0.json.gz"

	sess, err := session.NewSession()
	if err != nil {
		logErrorf("Unable to establish session in region %q, %v", os.Getenv("BUCKET_REGION"), err)
		return false
	}

	svc := s3.New(sess, &aws.Config{
		Region: aws.String(os.Getenv("BUCKET_REGION")),
	})

	input := &s3.SelectObjectContentInput{
		Bucket: aws.String(os.Getenv("BUCKET_NAME")),
		Key: aws.String(key),
		ExpressionType: aws.String(s3.ExpressionTypeSql),
		Expression: aws.String("SELECT * FROM S3Object s WHERE s.name = '" + publicIp + "'"),
		InputSerialization: &s3.InputSerialization{
			CompressionType: aws.String("GZIP"),
			JSON:            &s3.JSONInput{Type: aws.String("LINES")},
		},
		OutputSerialization: &s3.OutputSerialization{
			JSON: &s3.JSONOutput{RecordDelimiter: aws.String("\n")},
		},
	}

	out, err := svc.SelectObjectContent(input)
	if err != nil {
		logErrorf("Unable to select object content for key %q, %v", key, err)
		return false
	}
	defer out.EventStream.Close()

	for evt := range out.EventStream.Events() {
		switch e := evt.(type) {
		case *s3.RecordsEvent:
			log.Println(string(e.Payload))
			var record Record
			json.Unmarshal(e.Payload, &record)

			re := regexp.MustCompile("amazonaws|cloudfront")
			res := re.MatchString(record.Value)
			if res {
				return false
			}

			notifySlack("<!channel> :fishing_pole_and_fish: found a target: " + record.Value + " at: " + publicIp, "good")
			return true
		}
	}
	return false
}

func findTargetsOnShodan(publicIp string) bool {
	apiKey := os.Getenv("SHODAN_API_KEY")
	client, _ := shodan.GetClient(apiKey, http.DefaultClient, true)
	ctx := context.Background()
	hostSearch := search.HostParams{
		Minify: false,
		History: true,
		IP: publicIp,
	}

	result, err := client.Host(ctx, hostSearch)
	if err != nil {
		logErrorf("Error searching Shodan for %s: %v", publicIp, err)
		return false
	}

	hostnames := make(map[string]bool)

	for _, service := range result.Services {
		hostname := service.Shodan.Options.Hostname
		if len(hostname) > 0 {
			hostnames[hostname] = true
		}
	}

	uniqueHostnames := make([]string, 0, len(hostnames))
	for key := range hostnames {
		uniqueHostnames = append(uniqueHostnames, key)
	}

	for _, uniqueHostname := range uniqueHostnames {
		currentIPs, err := net.LookupIP(uniqueHostname)
		if err != nil {
			logErrorf("Unable to lookup IP %v", err)
		}

		for _, ipAddress := range currentIPs {
			if ipAddress.Equal(net.ParseIP(publicIp)) {
				notifySlack("<!channel> :fishing_pole_and_fish: found a target: " + uniqueHostname + " at: " + publicIp, "good")
				return true
			}
		}
	}

	return false
}

func notifySlack(message string, color string) {
	webhookUrl := os.Getenv("WEBHOOK_URL")

	attachment := slack.Attachment{
		Color:         color,
		Fallback:      message,
		AuthorName:    "eipfish",
		AuthorSubname: "AWS Lambda",
		AuthorLink:    "https://github.com/correkthorse",
		AuthorIcon:    "https://avatars3.githubusercontent.com/u/64679059",
		Text:          message,
		Footer:        "slack api",
		FooterIcon:    "https://platform.slack-edge.com/img/default_application_icon.png",
		Ts:            json.Number(strconv.FormatInt(time.Now().Unix(), 10)),
	}
	msg := slack.WebhookMessage{
		Attachments: []slack.Attachment{attachment},
	}

	slackErr := slack.PostWebhook(webhookUrl, &msg)
	if slackErr != nil {
		exitErrorf("Unable to post to webhook %v", slackErr)
	}
}

func logErrorf(msg string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, msg+"\n", args...)
	message := fmt.Sprintf(msg+"\n", args...)
	notifySlack("<!channel> :warning: " + message, "bad")
}

func exitErrorf(msg string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, msg+"\n", args...)
	message := fmt.Sprintf(msg+"\n", args...)
	notifySlack("<!channel> :rotating_light: " + message, "bad")
	os.Exit(1)
}
