regions=('us-east-1' 'us-west-2' 'ap-southeast-2' 'eu-west-1' 'eu-west-2')
for region in "${regions[@]}"
do
  echo "Checking addresses in $region"
  for eip in $(aws ec2 describe-addresses --region $region --query 'Addresses[].PublicIp' --output text)
  do
    echo "Releasing $eip from region $region"
    aws ec2 release-address --region $region --allocation-id $(aws ec2 describe-addresses --public-ip $eip --query Addresses[].AllocationId 2>/dev/null) 2>/dev/null
  done
done
