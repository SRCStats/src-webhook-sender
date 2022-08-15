package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var (
	database   string
	collection string
	wg         sync.WaitGroup
)

const (
	connectionStringEnv = "SRC_WEBHOOK_MONGODB_CONNECTION_STRING"
	databaseNameEnv     = "SRC_WEBHOOK_DATABASE"
	collectionNameEnv   = "SRC_WEBHOOK_INFO_COLLECTION"
)

type Webhook struct {
	WebhookUrl string `json:"WebhookUrl,omitempty"`
	Records    struct {
		Categories []string `json:"Categories,omitempty"`
		Users      []string `json:"Users,omitempty"`
		Events     string   `json:"Events,omitempty"`
	}
	Verification struct {
		Context string   `json:"Context,omitempty"`
		IDs     []string `json:"IDs,omitempty"`
		Events  []string `json:"Events,omitempty"`
	}
}

type Data struct {
	ID      string `json:"id,omitempty"`
	Order   int    `json:"order,omitempty"`
	New     bool   `json:"new,omitempty"`
	Weblink string `json:"weblink,omitempty"`
	Game    struct {
		Data struct {
			ID    string `json:"id,omitempty"`
			Names struct {
				International string `json:"international,omitempty"`
				Japanese      string `json:"japanese,omitempty"`
				Twitch        string `json:"twitch,omitempty"`
			} `json:"names,omitempty"`
			Abbreviation string   `json:"abbreviation,omitempty"`
			Platforms    []string `json:"platforms,omitempty"`
			Regions      []string `json:"regions,omitempty"`
			// im not sure this works
			Moderators []string `json:"moderators,omitempty"`
			Assets     struct {
				Trophy1st struct {
					URI string `json:"uri,omitempty"`
				} `json:"trophy-1st,omitempty"`
				Trophy2nd struct {
					URI string `json:"uri,omitempty"`
				} `json:"trophy-2nd,omitempty"`
				Trophy3rd struct {
					URI string `json:"uri,omitempty"`
				} `json:"trophy-3rd,omitempty"`
				Trophy4th struct {
					URI string `json:"uri,omitempty"`
				} `json:"trophy-4th,omitempty"`
			} `json:"assets,omitempty"`
		} `json:"data,omitempty"`
	} `json:"game,omitempty"`
	Level struct {
		Data struct {
			ID   string `json:"id,omitempty"`
			Name string `json:"name,omitempty"`
		} `json:"data,omitempty"`
	} `json:"level,omitempty"`
	Category struct {
		Data struct {
			ID            string `json:"id,omitempty"`
			Name          string `json:"name,omitempty"`
			Type          string `json:"type,omitempty"`
			Miscellaneous bool   `json:"miscellaneous,omitempty"`
			Variables     struct {
				Data []struct {
					ID       string      `json:"id,omitempty"`
					Name     string      `json:"name,omitempty"`
					Category interface{} `json:"category,omitempty"`
					Scope    struct {
						Type string `json:"type,omitempty"`
					} `json:"scope,omitempty"`
					Mandatory     bool                   `json:"mandatory,omitempty"`
					UserDefined   bool                   `json:"user-defined,omitempty"`
					Obsoletes     bool                   `json:"obsoletes,omitempty"`
					Values        map[string]interface{} `json:"values,omitempty"`
					IsSubcategory bool                   `json:"is-subcategory,omitempty"`
				} `json:"data,omitempty"`
			} `json:"variables,omitempty"`
		} `json:"data,omitempty"`
	} `json:"category,omitempty"`
	Times struct {
		Primary          float64 `json:"primary_t,omitempty"`
		RealTime         float64 `json:"realtime_t,omitempty"`
		RealTimeLoadless float64 `json:"realtime_noloads_t,omitempty"`
		InGameTime       float64 `json:"ingame_t,omitempty"`
	}
	Videos struct {
		Links []struct {
			URI string `json:"uri,omitempty"`
		} `json:"links,omitempty"`
	} `json:"videos,omitempty"`
	Comment string `json:"comment,omitempty"`
	Status  struct {
		Status     string    `json:"status,omitempty"`
		Examiner   string    `json:"examiner,omitempty"`
		Reason     string    `json:"reason,omitempty"`
		VerifyDate time.Time `json:"verify-date,omitempty"`
	} `json:"status,omitempty"`
	Players struct {
		Data []struct {
			ID      string `json:"id,omitempty"`
			Weblink string `json:"weblink,omitempty"`
			Names   struct {
				International string `json:"international,omitempty"`
				Japanese      string `json:"japanese,omitempty"`
			} `json:"names,omitempty"`
			Name   string `json:"name,omitempty"`
			Assets struct {
				Icon struct {
					URI string `json:"uri,omitempty"`
				} `json:"icon,omitempty"`
				Image struct {
					URI string `json:"uri,omitempty"`
				} `json:"image,omitempty"`
			} `json:"assets,omitempty"`
		} `json:"data,omitempty"`
	} `json:"players,omitempty"`
	Date      string    `json:"date,omitempty"`
	Submitted time.Time `json:"submitted,omitempty"`
	System    struct {
		Platform string `json:"platform,omitempty"`
		Emulated bool   `json:"emulated,omitempty"`
		Region   string `json:"region,omitempty"`
	} `json:"system,omitempty"`
	Values map[string]string `json:"values,omitempty"`
	Region struct {
		Data struct {
			ID   string `json:"id,omitempty"`
			Name string `json:"name,omitempty"`
		} `json:"data,omitempty"`
	} `json:"region,omitempty"`
	Platform struct {
		Data struct {
			ID   string `json:"id,omitempty"`
			Name string `json:"name,omitempty"`
		} `json:"data,omitempty"`
	} `json:"platform,omitempty"`
}

type Response struct {
	Data       []Data `json:"data,omitempty"`
	Pagination struct {
		Offset int `json:"offset,omitempty"`
		Max    int `json:"max,omitempty"`
		Size   int `json:"size,omitempty"`
	} `json:"pagination,omitempty"`
	Scope string `json:"scope,omitempty"`
}

func main() {
	listenAddr := ":8080"
	if val, ok := os.LookupEnv("FUNCTIONS_CUSTOMHANDLER_PORT"); ok {
		listenAddr = ":" + val
	}
	http.HandleFunc("/api/SendWebhook", runsHandler)
	log.Fatal(http.ListenAndServe(listenAddr, nil))
}

func runsHandler(w http.ResponseWriter, r *http.Request) {
	// todo: handle invalid bodies
	body := r.Body
	defer body.Close()
	b, err := io.ReadAll(body)
	if err != nil {
		log.Fatal(err)
	}
	var data []Data
	err = json.Unmarshal(b, &data)
	if err != nil {
		log.Fatal(err)
	}
	webhooks := GetWebhooks()
	switch data[0].Status.Status {
	case "verified":
		HandleVerified(&data, &webhooks)
		break
	case "new":
		break
	case "rejected":
		break
	}
}

func HandleVerified(data *[]Data, webhooks *[]Webhook) {
	wg.Add(1)
	for _, run := range *data {
	nextWebhook:
		// todo: add handling for listings of categories and users on same webhook
		for _, webhook := range *webhooks {
			for _, category := range webhook.Records.Categories {
				if category == run.Category.Data.ID {
					fmt.Print(category)
					if run.Players.Data[0].Names.International == "" {
						continue nextWebhook
					}
					SendWebhook(&webhook, &run, "verified", 0)
					continue nextWebhook
				}
			}
			for i, player := range run.Players.Data {
				for _, wPlayer := range webhook.Records.Users {
					if wPlayer == player.ID {
						SendWebhook(&webhook, &run, "verified", i)
						continue nextWebhook
					}
				}
			}
		}
	}
	wg.Done()
}

func SendWebhook(webhook *Webhook, run *Data, scope string, playerIndex int) {
	wg.Add(1)
	time.Sleep(1 * time.Second)
	switch scope {
	case "verified":
		var author string
		if len(run.Players.Data) > 2 {
			author = fmt.Sprintf("%v and %v others", run.Players.Data[playerIndex].Names.International, len(run.Players.Data)-1)
		} else if len(run.Players.Data) > 1 {
			author = fmt.Sprintf("%v and %v other", run.Players.Data[playerIndex].Names.International, len(run.Players.Data)-1)
		} else {
			author = run.Players.Data[playerIndex].Names.International
		}
		var category, variables, players, iconUrl, runTime string
		for key, value := range run.Values {
			for _, variable := range run.Category.Data.Variables.Data {
				if variable.ID == key {
					for varId, varVal := range variable.Values["choices"].(map[string]interface{}) {
						if varId == value {
							if variable.IsSubcategory {
								if category != "" {
									category += fmt.Sprintf(", %v", varVal)
								} else {
									category = fmt.Sprintf("%v (%v", run.Category.Data.Name, varVal)
								}
							} else {
								if variables != "" {
									variables += fmt.Sprintf(", %v: %v", variable.Name, varVal)
								} else {
									variables = fmt.Sprintf(" (%v: %v", variable.Name, varVal)
								}
							}
						}
					}
				}
			}
		}
		if len(run.Players.Data) > 1 {
			for _, player := range run.Players.Data {
				if players != "" {
					if player.Name == "" {
						players += fmt.Sprintf(", %v", player.Names.International)
					} else {
						players += fmt.Sprintf(", %v", player.Name)
					}
				} else {
					if player.Name == "" {
						players = player.Names.International
					} else {
						players = player.Name
					}
				}
			}
		}
		dur := time.Duration(run.Times.Primary) * time.Second
		runTime = dur.String()
		if category != "" {
			category += ")"
		} else {
			category = run.Category.Data.Name
		}
		if variables != "" {
			variables += ")"
		}
		if iconUrl == "" {
			iconUrl = "" //run.Game.Data.Assets.Trophy1st.URI
		}
		if runTime == "" {
			runTime = "0s"
		}
		fields := []map[string]interface{}{
			{
				"name":   "Category",
				"value":  category,
				"inline": true,
			},
			{
				"name":   "Time",
				"value":  runTime,
				"inline": true,
			},
		}
		if players != "" {
			fields = append([]map[string]interface{}{
				{
					"name":   "Players",
					"value":  players,
					"inline": true,
				},
			}, fields...)
		}
		if variables != "" {
			fields = append(fields, map[string]interface{}{
				"name":   "Variables",
				"value":  variables,
				"inline": true,
			})
		}
		if run.Level.Data.Name != "" {
			fields = append([]map[string]interface{}{
				{
					"name":   "Level",
					"value":  run.Level.Data.Name,
					"inline": true,
				},
			}, fields...)
		}
		jsonBody := map[string]interface{}{
			"content": nil,
			"embeds": []map[string]interface{}{
				{
					"author": map[string]interface{}{
						"name":     run.Players.Data[playerIndex].Names.International,
						"url":      run.Players.Data[playerIndex].Weblink,
						"icon_url": run.Players.Data[playerIndex].Assets.Image.URI,
					},
					"color":       "15899392",
					"title":       fmt.Sprintf("New personal best by %v!", author),
					"description": fmt.Sprintf("**%v** got a new personal best in **%v**!", author, run.Game.Data.Names.International),
					"fields":      fields,
					"url":         run.Weblink,
					"footer": map[string]interface{}{
						"text":     fmt.Sprintf("They're now %v place!", "(run place)"),
						"icon_url": iconUrl,
					},
				},
			},
			"attachments": nil,
		}
		body, err := json.Marshal(jsonBody)
		if err != nil {
			log.Printf("Error while marshalling webhook body!\n%v", err)
		}
		res, err := http.Post(webhook.WebhookUrl, "application/json", bytes.NewBuffer(body))
		if err != nil {
			log.Printf("Error while sending webhook!\n%v", err)
		} else {
			log.Printf("Webhook sent to %v!", webhook.WebhookUrl)
		}
		if res.StatusCode == 404 {
			log.Printf("Webhook %v is not found!", webhook.WebhookUrl)
			DeleteWebhook(*webhook)
		}
		if res.StatusCode == 429 {
			log.Printf("Webhook %v is over rate limit!", webhook.WebhookUrl)
		}
		break
	case "new":
		break
	case "rejected":
		break
	}
	wg.Done()
}

func Connect() *mongo.Client {
	connectionString := os.Getenv(connectionStringEnv)
	if connectionString == "" {
		log.Fatal("The database connection string variable is missing!")
	}
	database = os.Getenv(databaseNameEnv)
	if database == "" {
		log.Fatal("The database name variable is missing!")
	}
	collection = "webhook-list"
	if collection == "" {
		log.Fatal("The collection name variable is missing!")
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	options := options.Client().ApplyURI(connectionString).SetDirect(true)
	c, _ := mongo.NewClient(options)

	err := c.Connect(ctx)
	if err != nil {
		log.Fatalf("Connection couldn't be initialized!\n%v", err)
	}
	err = c.Ping(ctx, nil)
	if err != nil {
		log.Fatalf("Connection didn't respond back!\n%v", err)
	}
	return c
}

func GetWebhooks() []Webhook {
	c := Connect()
	defer c.Disconnect(context.TODO())
	collection := c.Database(database).Collection(collection)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	var webhooks []Webhook
	cur, err := collection.Find(ctx, bson.D{})
	if err != nil {
		log.Fatalf("Error while finding webhooks!\n%v", err)
	}
	defer cur.Close(ctx)
	for cur.Next(ctx) {
		var result Webhook
		err := cur.Decode(&result)
		if err != nil {
			log.Fatalf("Error while decoding webhooks!\n%v", err)
		}
		webhooks = append(webhooks, result)
	}
	if err := cur.Err(); err != nil {
		log.Fatalf("Error while iterating webhooks!\n%v", err)
	}
	return webhooks
}

func DeleteWebhook(webhook Webhook) {
	c := Connect()
	defer c.Disconnect(context.TODO())
	collection := c.Database(database).Collection(collection)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	result, err := collection.DeleteOne(ctx, bson.M{"WebhookUrl": webhook.WebhookUrl})
	if err != nil {
		log.Printf("Error while deleting webhook!\n%v", err)
	}
	log.Printf("Deleted %v webhook with url %v", result.DeletedCount, webhook.WebhookUrl)
}
