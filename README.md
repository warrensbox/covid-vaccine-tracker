# Covid Vaccination Notifier

A lightweight application that send you a notification when a vaccine appointment becomes available near you. Get notified on: 
- Slack
- Teams
- Email
- SMS

### Installation 
Install the binary as a AWS lambda function or simply run it on your machine.

### Workflow
<img src="https://s3.us-east-2.amazonaws.com/kepler-images/warrensbox/covid-vaccine-tracker/covid-vaccine-tracker-workflow-white-bg.svg" alt="drawing" style="width: 370px;"/>

1. CloudWatch will periodically trigger lambda.
1. The lambda function (Notifier app) will call the following API: `https://www.vaccinespotter.org/api/v0/states/<STATE>.json`
1. With the returned payload from the API, we will check against DynamoDB if the alert has been sent before. If it's the same as the previous alert, the function does nothing.
1. If the alert is new and is different than the previous alert, the function will trigger the SNS Topic.
1. All resources subscribing to the SNS topic will receive the alert.

### Follow step-by-step instructions to install Covid notifier on your AWS account
#### 1. Create IAM Policy 
- Navigate to the IAM Page on AWS console   
- Create new policy - `covid-vaccine-all-lambda`  
- Update `<update-account-number-here>` with your AWS account number   
```    "Version": "2021-03-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Action": [
                "dynamodb:BatchGetItem",
                "dynamodb:GetItem",
                "dynamodb:Query",
                "dynamodb:Scan",
                "dynamodb:BatchWriteItem",
                "dynamodb:PutItem",
                "dynamodb:UpdateItem"
            ],
            "Resource": "arn:aws:dynamodb:us-east-1:<update-account-number-here>:table/Covid"
        },
        {
            "Effect": "Allow",
            "Action": [
                "logs:CreateLogStream",
                "logs:PutLogEvents"
            ],
            "Resource": "arn:aws:logs:us-east-1:<update-account-number-here>:*"
        },
        {
            "Effect": "Allow",
            "Action": "logs:CreateLogGroup",
            "Resource": "*"
        }
    ]
}
```

#### 2. Create IAM Role
- Navigate to the IAM Page on AWS console   
- Create new Role
- For 'Choose a use case', select Lambda  
- Filter for the policy you've created in the previous step `covid-vaccine-all-lambda`
- Name the new role - `covid-vaccine-all-role`

#### 3. Create SNS Topic
- Navigate to the SNS Page on AWS console
- Create topic
- Type: Standard
- Name: `covid-vaccine-notifier`
- You will need the `Topic ARN` for the next step. Copy for later use
#### 4. Create Dynamo Table
- Navigate to the DynamoDb Page on AWS console
- Create new table
- Table name: `Covid`
- Primary key* : Partition key: `Source` Type: `string`
#### 5. Create Lambda Function
- Navigate to the Lambda Page on AWS console
- Create new lambda function
- Function name: `covid-vaccine-notifier`
- Runtime: `Go 1.x`
- Change default execution role: `Use an existing role`
- Existing role: `covid-vaccine-all-role`
- Code source: upload from: .zip file
- Upload the zip file from: [github here](https://github.com/warrensbox/covid-vaccine-tracker/releases) 
- Update Runtime setting to: `bin/covid-vaccine-notifier`
- Navigate to the `Configuration` tab
- Navigate to `Environment variables`
- Insert the following environment variables:
- MUTE: hyvee (the companies you would like to mute)	 
- RANGE_A: 00000 (starting range of zipcode - leave blank if you want the whole state)	
- RANGE_B: 99999 (ending range of zipcode - leave blank if you want the whole state)
- SOURCE: covid-vaccine-notifier (you don't have to change this)
- STATE: IA (match the state you're living)
- TABLE_ID: 2019 (you don't have to change this)	
- TABLE_NAME: Covid (you don't have to change this)	 
- TOPIC_ARN:  (paste the topic ARN from the previous step)	
- AWS_REGION: Default is us-east-1   
- See example:   
<img src="https://s3.us-east-2.amazonaws.com/kepler-images/warrensbox/covid-vaccine-tracker/covid-vaccine-notifier-env-vars.png" alt="drawing" width="470"/>

#### 6. Create CloudWatch Rule
- Navigate to the CloudWatch Page on AWS console
- Navigate to Events-> Rules
- Create Rule
- Step 1: Event Source. Choose `Schedule`
- Enter the rate you want the API to be checked. Ideally it would be 5 minutes
- Target: Choose `Lambda function` 
- Function: `covid-vaccine-notifier`
#### 7. Create Subscription
- Navigate to the SNS Page on AWS console
- On side bar, select `Subscription`
- Next, `Create subscription`
- On dropdown - Select the SNS Topic ARN , created in previous step - `arn:aws:sns:us-east-1:<update-account-number-here>:covid-vaccine-notifier`
- Protocol - Choose SMS for text message notification or Email for email notification
- Both Slack and Teams channel should have an option to send emails to that channel.

### How to get email for Slack
<img src="https://s3.us-east-2.amazonaws.com/kepler-images/warrensbox/covid-vaccine-tracker/slack-email.png" alt="drawing" width="350"/>

### How to get email for Team
<img src="https://s3.us-east-2.amazonaws.com/kepler-images/warrensbox/covid-vaccine-tracker/teams-email.png" alt="drawing" width="350"/>

### Issues
https://github.com/warrensbox/covid-vaccine-tracker/issues