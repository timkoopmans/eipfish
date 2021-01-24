Hello, I am an independent security researcher, and this is a courtesy message to let you know that the subdomain $TARGET has Name Server (NS) records that point to an AWS EC2 Elastic IP address you no longer control.

I have allocated this address for research, but your NS records still point to it. As such I have takeover control for this subdomain:

dig $TARGET

; <<>> DiG 9.10.6 <<>> $TARGET
;; global options: +cmd
;; Got answer:
;; ->>HEADER<<- opcode: QUERY, status: NOERROR, id: 52085
;; flags: qr rd ra; QUERY: 1, ANSWER: 1, AUTHORITY: 0, ADDITIONAL: 1

;; OPT PSEUDOSECTION:
; EDNS: version: 0, flags:; udp: 512
;; QUESTION SECTION:
;$TARGET.       IN      A

;; ANSWER SECTION:
$TARGET. 13802  IN      A       13.238.199.191

;; Query time: 71 msec
;; SERVER: 8.8.8.8#53(8.8.8.8)
;; WHEN: Sun Jan 24 22:26:43 AEDT 2021
;; MSG SIZE  rcvd: 68

For proof of concept, please visit http://$TARGET
There will also be a snapshot of this page at https://web.archive.org/web/20210124112139/http://$TARGET/

This is considered a high impact issue as I can run any service on any port at this subdomain. I can potentially read cookies set from the main domain, perform cross-site scripting, or circumvent content security policies, thereby enabling the ability to capture protected information or send malicious content to unsuspecting users.

For more information about subdomain takeovers please refer to:

https://developer.mozilla.org/en-US/docs/Web/Security/Subdomain_takeovers

To fix this issue you can:

1. Remove the dangling NS record pointing to this Elastic IP address no longer under your control.
2. Recover the Elastic IP address from the pool after I have released the association in near future.

Regards,
Tim
