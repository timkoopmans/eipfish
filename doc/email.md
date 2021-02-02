Hello, I am an independent security researcher, and this is a courtesy message to let you know that the subdomain blog.exa.com.au has Name Server (NS) records that point to an AWS EC2 Elastic IP address you no longer control.

I have allocated this address for research, but your NS records still point to it. As such I have takeover control for this subdomain:

; <<>> DiG 9.10.6 <<>> blog.exa.com.au
;; global options: +cmd
;; Got answer:
;; ->>HEADER<<- opcode: QUERY, status: NOERROR, id: 48870
;; flags: qr rd ra; QUERY: 1, ANSWER: 1, AUTHORITY: 0, ADDITIONAL: 1

;; OPT PSEUDOSECTION:
; EDNS: version: 0, flags:; udp: 4096
;; QUESTION SECTION:
;blog.exa.com.au.               IN      A

;; ANSWER SECTION:
blog.exa.com.au.        300     IN      A       52.62.3.228

;; Query time: 38 msec
;; SERVER: 172.21.0.2#53(172.21.0.2)
;; WHEN: Tue Jan 26 20:45:03 AEDT 2021
;; MSG SIZE  rcvd: 60

# Proof of Concept
For proof of concept, I have run a server at http://blog.exa.com.au

You will see "VGhpcyBpcyBhIHN1YmRvbWFpbiB0YWtlb3ZlciBieSBAY29ycmVrdGhvcnNlCg==" which is base64 encoded for "This is a subdomain takeover by @correkthorse"

I have created a snapshot of this proof of concept on the Internet Archive Wayback Machine at
https://web.archive.org/web/20210126094906/http://blog.exa.com.au/

# Impact
This is considered a high impact issue as I can run any service on any port at this subdomain. I can potentially read cookies set from the main domain, perform cross-site scripting, or circumvent content security policies, thereby enabling the ability to capture protected information or send malicious content to unsuspecting users.

# Suggested Fix
To fix this issue you can:

1. Remove the dangling NS record pointing to this Elastic IP address no longer under your control.
2. Recover the Elastic IP address from the pool after I have released the association in near future.

For more information about subdomain takeovers please refer to:

https://developer.mozilla.org/en-US/docs/Web/Security/Subdomain_takeovers


Regards,
Tim
@correkthorse
