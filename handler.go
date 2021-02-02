package main

import (
	"encoding/json"
	"fmt"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"log"
	"net"

	"net/http"
	"regexp"
	"strconv"
	"time"

	"context"
	"github.com/flowchartsman/retry"
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
	region := "ap-southeast-2"
	findTargetsOnShodan(region, publicIp)
}

func Handler(event Event) (Result, error){
	region := event.Region
	publicIp, allocationId := allocateAddress(region)

	log.Printf("Checking %s from allocation ID %s in region %s\n", publicIp, allocationId, region)

	if findTargetsOnShodan(region, publicIp) {
		return Result{Message: fmt.Sprintf("found target on %s in region %s", publicIp, region)}, nil
	} else {
		retrier := retry.NewRetrier(4, 100 * time.Millisecond, 15 * time.Second)

		err := retrier.Run(func() error {
			err := releaseAddress(region, publicIp, allocationId)
			if err != nil {
				return err
			}

			return nil
		})
		if err != nil {
			return Result{Message: fmt.Sprintf("problem releasing %s in region %s", publicIp, region)}, err
		}

		return Result{Message: fmt.Sprintf("no matches on %s in region %s", publicIp, region)}, nil
	}

	return Result{Message: fmt.Sprintf("processed %s in region %s", publicIp, region)}, nil
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

func releaseAddress(region string, publicIp string, allocationId string) error {
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
				log.Println("Allocation ID %s does not exist", allocationId)
				return aerr
			}
			log.Println("Unable to release IP address for allocation %s: %v", allocationId, err)
			return err
		}

		log.Printf("Released %s from allocation ID %s\n", publicIp, allocationId)
		return nil
	} else {
		log.Printf("No allocation ID to release for %s\n", publicIp)
		return nil
	}
}

func findTargetsOnShodan(region string, publicIp string) bool {
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
		log.Println("Unable to search Shodan for %s: %v", publicIp, err)
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
		re := regexp.MustCompile("amazonaws|cloudfront")
		res := re.MatchString(uniqueHostname)
		if res {
			continue
		}

		currentIPs, err := net.LookupIP(uniqueHostname)
		if err != nil {
			log.Printf("Fish got away on EIP %s: %v", publicIp, err)
		}

		for _, ipAddress := range currentIPs {
			if ipAddress.Equal(net.ParseIP(publicIp)) {
				notifySlack("<!channel> :fishing_pole_and_fish: caught a fish in " + region + " at " + uniqueHostname + " on " + publicIp, "good")
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
