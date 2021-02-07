![](eipfish.png)

# EIP Fishing

This is an AWS Lambda that runs a small Go binary on a schedule. 
Each execution of the binary will allocate an Elastic IP (EIP) in the region you specify. 
It checks for historical records using the [Shodan API](https://developer.shodan.io/api).

If there are any matches, it retains the EIP for further use, otherwise it releases the allocation back to the pool.

## Setup

Make sure you provide your own `.env` file with following keys:    

    WEBHOOK_URL="<your own incoming webhook URL for Slack>"
    SHODAN_API_KEY="<paid subscription to Shodan>"

Install serverless:

    npm install serverless -g

Deploy using your own AWS environment:

    make deploy

Monitor via slack:

![](demo.png)

## tl;dr

This is a simple lambda that I am using to help discover dangling NS records related to AWS EIPs for bug bounty programs.

### Output

    START RequestId: 56cf9c1c-551e-19db-3706-5a95a9480d75 Version: $LATEST
    Checking 54.186.54.63 from allocation ID eipalloc-0be40fc401b82cfec
    {"timestamp":"1609308060","name":"54.186.54.63","value":"ec2-54-186-54-63.us-west-2.compute.amazonaws.com","type":"ptr"}

    Released 54.186.54.63 from allocation ID eipalloc-0be40fc401b82cfec
    END RequestId: 56cf9c1c-551e-19db-3706-5a95a9480d75
    REPORT RequestId: 56cf9c1c-551e-19db-3706-5a95a9480d75  Init Duration: 387.81 ms        Duration: 5264.54 ms    Billed Duration: 5300 ms        Memory Size: 128 MB     Max Memory Used: 35 MB

    {"RESULT:":"no matches on 54.186.54.63"}

### Check for bounty programs

    curl -s https://raw.githubusercontent.com/disclose/diodb/master/program-list.json | \
        jq -r '.[] | select(.program_name|test("mastercard";"i"))'    