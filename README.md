# Covid Vaccination Notifier

A lightweight application that send you a notification when a vaccine becomes available near you. Get notified on: 
- Slack
- Teams
- Email
- SMS

### Installation 
Install the binary as a AWS lambda function or simply on it on your machine.

### Workflow
<img src="https://s3.us-east-2.amazonaws.com/kepler-images/warrensbox/covid-vaccine-tracker/covid-vaccine-tracker-workflow-white-bg.svg" alt="drawing" style="width: 370px;"/>

1. CloudWatch Rules will trigger lambda.
1. The lambda function(Notifier app) will call the following API: `https://www.vaccinespotter.org/api/v0/states/<STATE>.json`
1. The returned payload from the API will be hashed and checked if the alert had been sent before. If the hash matches the previously sent alert, the function does nothing.
1. If the alert if different than the previous alert, the function will trigger an SNS Topic.
1. All resources subcribing to the SNS topic will receive the alert.



### Step by step instruction to install notifier on your AWS account
#### Create Lambda Function



