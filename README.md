# SRC Webhook Sender
A partner program designed to send webhooks to Discord webhooks based on Speedrun.com run data received via an HTTP POST request. Designed for a serverless model running alongside [a function for fetching the runs from Speedrun.com]((https://github.com/SRCStats/src-webhook-fetcher)).

# Configuration
## **IMPORTANT**
This application is not intended to be used in production yet. It currently misses features like leaderboard placements, milliseconds, and handling for the Verification category of filters. It also needs a better system for rate limiting, as it currently only sends 1 Discord webhook per second, which doesn't scale very well.

This function is designed for Azure Functions running on a Windows machine. Setting up Azure Functions is not necessary for developing and debugging the code, and so there will be no guide on how to configure that.

Run the following commands in the desired location (other than $GOPATH):
```bat
git clone https://github.com/SRCStats/src-webhook-sender.git
cd src-webhook-sender
```
Configure a new MongoDB instance, or use an existing one. ([Azure Cosmos DB](https://azure.microsoft.com/en-us/services/cosmos-db/) w/ MongoDB API is used in production, but any will do.)
Run the following commands to set the environment variables:

*The {Value} fields are placeholders.*
### Windows
```bat
setx SRC_WEBHOOK_MONGODB_CONNECTION_STRING {Connection_String}
setx SRC_WEBHOOK_DATABASE {Database_Name} &:: "srcstats" Recommended
setx SRC_WEBHOOK_INFO_COLLECTION {Collection_Name} &:: "webhook-list" Recommended
```
### Mac/Linux
```bash
# This is temporary for your current terminal session, add these commands to your .bashrc or .bash_profile to persist across sessions
export SRC_WEBHOOK_MONGODB_CONNECTION_STRING={Connection_String}
export SRC_WEBHOOK_DATABASE={Database_Name} # "srcstats" Recommended
export SRC_WEBHOOK_COLLECTION={Collection_Name} # "webhook-last-runs" Recommended
```
You should also clone and configure [SRC Webhook Fetcher](https://github.com/SRCStats/src-webhook-fetcher), with the SRC_WEBHOOK_RUN_URI variable configured to use the same localhost and port that this uses (:8080 by default). You can also use any tool for sending HTTP POST requests with a body of the proper JSON format for runs.

Ensure your database has at least one entry in the database and collection specified in the variables above, with the following format:
```json
{
	"WebhookUrl" : "https://discord.com/api/webhooks/xxxxxxxxxxxxx/xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
	"Records" : {
		"Categories" : [
			"<id>",
			"<for>",
			"<each>",
			"<category>"
		],
		"Users" : [
			"<id>",
			"<for>",
			"<each>",
			"<user>"
		],
		"Events" : "<wr|pb|all>" // "all" recommended
	},
	"Verification" : null // not supported yet
}
```
Finally, run the following go commands to finish the configuration and run the project:
```
go get ./...
go run sender.go
```

# License
This project is licensed under the Anti-Capitalist Software License. See [LICENSE](LICENSE) for more information.