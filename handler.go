package main

import (
	"encoding/json"
	"fmt"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/flowchartsman/retry"
	"log"
	"net"

	"net/http"
	"regexp"
	"strconv"
	"time"

	"context"
	"github.com/bobesa/go-domain-util/domainutil"
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

type DiscloseProgram struct {
	ProgramName string `json:"program_name"`
	PolicyURL string `json:"policy_url"`
	ContactURL string  `json:"contact_url"`
	LaunchDate string  `json:"launch_date"`
	OffersBounty string  `json:"offers_bounty"`
	//OffersSwag string  `json:"offers_swag"`
	HallOfFame string  `json:"hall_of_fame"`
	SafeHarbor string  `json:"safe_harbor"`
	PublicDisclosure string  `json:"public_disclosure"`
	PgpKey string  `json:"pgp_key"`
	Hiring string  `json:"hiring"`
	SecurityTextURL string  `json:"securitytxt_url"`
	PreferredLanguages string  `json:"preferred_languages"`
	PolicyURLStatus string  `json:"policy_url_status"`
}

func main() {
	lambda.Start(Handler)
	//Debug()
}

func Debug() {
	publicIp := "52.6.170.149"
	region := "us-east-1"
	target := findTargetsOnShodan(region, publicIp)
	if target != "" {
		bounty := findBountiesOnDisclose(target)

		if bounty {
			notifySlack("<!channel> :fishing_pole_and_fish: caught a *BIG* fish " + region + " at " + target + " on " + publicIp, "bad")
		} else {
			notifySlack(":fishing_pole_and_fish: caught a fish in " + region + " at " + target + " on " + publicIp, "good")
		}
	}
}

func Handler(event Event) (Result, error){
	region := event.Region
	publicIp, allocationId := allocateAddress(region)

	log.Printf("Checking %s from allocation ID %s in region %s\n", publicIp, allocationId, region)

	target := findTargetsOnShodan(region, publicIp)
	if target != "" {
		bounty := findBountiesOnDisclose(target)

		if bounty {
			notifySlack("<!channel> :fishing_pole_and_fish: caught a *BIG* fish " + region + " at " + target + " on " + publicIp, "bad")
			return Result{Message: fmt.Sprintf("found target on %s in region %s", publicIp, region)}, nil
		} else {
			notifySlack(":fishing_pole_and_fish: caught a fish in " + region + " at " + target + " on " + publicIp, "good")
		}
	}
	retryLoop := retry.NewRetrier(4, 100 * time.Millisecond, 15 * time.Second)
	err := retryLoop.Run(func() error {
		err := releaseAddress(region, publicIp, allocationId)
		if err != nil {
			return err
		}
		return nil
	})
	return Result{Message: fmt.Sprintf("no matches on %s in region %s", publicIp, region)}, err
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

func findTargetsOnShodan(region string, publicIp string) string {
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
		return ""
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
				return uniqueHostname
			}
		}
	}

	return ""
}

func findBountiesOnDisclose(target string) bool {
	domain := domainutil.Domain(target)
	apiURL := "https://raw.githubusercontent.com/disclose/diodb/master/program-list.json"
	resp, err := http.Get(apiURL)
	if err != nil {
		log.Println("Unable to get Disclose for %s: %v", domain, err)
		return false
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Println("Unable to receive Disclose for %s: %v", domain, err)
		return false
	}

	programs := make([]DiscloseProgram, 0)
	decoder := json.NewDecoder(resp.Body)
	err = decoder.Decode(&programs)

	if err != nil {
		log.Println("Unable to parse Disclose for %s: %v", domain, err)
		return false
	}

	for _, program := range programs {
		domainRE := regexp.MustCompile(domain)
		ProgramNameMatch := domainRE.MatchString(program.ProgramName)
		if ProgramNameMatch {
			return true
		}
		PolicyURLMatch := domainRE.MatchString(program.PolicyURL)
		if PolicyURLMatch {
			return true
		}
		ContactURLMatch := domainRE.MatchString(program.ContactURL)
		if ContactURLMatch {
			return true
		}
		SecurityTextURLMatch := domainRE.MatchString(program.SecurityTextURL)
		if SecurityTextURLMatch {
			return true
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

func exitErrorf(msg string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, msg+"\n", args...)
	os.Exit(1)
}
