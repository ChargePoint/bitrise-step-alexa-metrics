package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	chart "github.com/wcharczuk/go-chart/v2"
)

var httpClient = &http.Client{Timeout: 200 * time.Second}

var metrics = [...]string{
	"uniqueCustomers",
	"totalEnablements",
	"totalEnablements",
	"successfulUtterances",
	"failedUtterances",
	"totalSessions",
	"successfulSessions",
	"incompleteSessions",
	"userEndedSessions",
	"skillEndedSessions"}

func lwaOAuthURL() string {
	return "https://api.amazon.com/auth/o2/token"
}

func metricsSMAPIURL(skillID string, startTime string, endTime string, metric string) string {
	return "https://api.amazonalexa.com/v1/skills/" + skillID + "/metrics?startTime=" + startTime + "&endTime=" + endTime + "&period=P1D&metric=" + metric + "&stage=live&skillType=custom&locale=en-US"
}

func getenv(key, fallback string) string {
	value := os.Getenv(key)
	if len(value) == 0 {
		return fallback
	}
	return value
}

type AuthenticateResponse struct {
	Access_token  string `json:"access_token"`
	Expires_in    int    `json:"expires_in"`
	Token_type    string `json:"token_type"`
	Refresh_token string `json:"refresh_token"`
}

type MetricsResponse struct {
	Metric     string    `json:"metric"`
	Timestamps []string  `json:"timestamps"`
	Values     []float64 `json:"values"`
}

func getLWAAccessToken(clientID string, clientSecret string, refreshToken string, target interface{}) error {
	var bodyString = "grant_type=refresh_token"
	bodyString += "&client_id=" + clientID
	bodyString += "&client_secret=" + clientSecret
	bodyString += "&refresh_token=" + refreshToken

	body := strings.NewReader(bodyString)
	req, err := http.NewRequest("POST", lwaOAuthURL(), body)
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := httpClient.Do(req)
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	return json.NewDecoder(resp.Body).Decode(target)
}

func formatTimeDate(t time.Time) string {
	return fmt.Sprintf("%d-%02d-%02dT%02d:%02d:%02dZ", t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second())
}

func getSkillMetric(skillID string, metric string, accessToken string, target interface{}) error {
	endTime := time.Now()
	startTime := endTime.AddDate(0, 0, -7)

	url := metricsSMAPIURL(skillID, formatTimeDate(startTime), formatTimeDate(endTime), metric)

	var bodyString = ""
	body := strings.NewReader(bodyString)

	req, err := http.NewRequest("GET", url, body)

	authorization_value := "Bearer " + accessToken
	req.Header.Set("Authorization", authorization_value)

	resp, err := httpClient.Do(req)
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}

	defer resp.Body.Close()

	responseData, sErr := ioutil.ReadAll(resp.Body)
	if sErr != nil {
		log.Fatal(err)
		os.Exit(1)
	}

	return json.Unmarshal(responseData, &target)
}

func generateMetricChart(deployDir string, metricResponse MetricsResponse) {
	series := make([]chart.Series, 1)
	count := len(metricResponse.Values)
	xValues := make([]time.Time, count)
	for i := 0; i < count; i++ {
		xValues[i], _ = time.Parse(time.RFC3339, metricResponse.Timestamps[i])
	}

	series[0] = chart.TimeSeries{
		Name:    metricResponse.Metric,
		XValues: xValues,
		YValues: metricResponse.Values,
	}

	graph := chart.Chart{
		Background: chart.Style{
			Padding: chart.NewBox(20, 20, 20, 20),
		},
		XAxis: chart.XAxis{
			Name: "Time",
		},
		YAxis: chart.YAxis{
			Name: "Value",
		},
		Series: series,
	}
	filepath := deployDir + "/" + metricResponse.Metric + ".png"
	f, _ := os.Create(filepath)
	defer f.Close()
	graph.Render(chart.PNG, f)
	fmt.Println(filepath)
}

func main() {
	fmt.Println("Get the LWA access token")

	lwaClientID := getenv("lwa_client_id", "")
	lwaClientSecret := getenv("lwa_client_secret", "")
	lwaRefreshToken := getenv("lwa_refresh_token", "")
	skillID := getenv("custom_skill_id", "")
	deployDir := getenv("BITRISE_DEPLOY_DIR", "")

	if lwaClientID == "" {
		fmt.Println("LWA Client ID is required")
		os.Exit(1)
	}

	if lwaClientSecret == "" {
		fmt.Println("LWA Client secret is required")
		os.Exit(1)
	}

	if lwaRefreshToken == "" {
		fmt.Println("LWA refresh token is required")
		os.Exit(1)
	}

	if skillID == "" {
		fmt.Println("Skill ID is required")
		os.Exit(1)
	}

	if deployDir == "" {
		fmt.Println("Deploy directory not found")
		os.Exit(1)
	}

	auth := AuthenticateResponse{}
	getLWAAccessToken(lwaClientID, lwaClientSecret, lwaRefreshToken, &auth)

	fmt.Println("LWA Access Token", auth.Access_token)

	for i := 0; i < 1; i++ {
		metricResponse := MetricsResponse{}
		getSkillMetric(skillID, metrics[i], auth.Access_token, &metricResponse)

		fmt.Println("Number of " + metricResponse.Metric + " on each day last week")
		for i := 0; i < len(metricResponse.Values); i++ {
			fmt.Println(metricResponse.Timestamps[i], metricResponse.Values[i])
		}

		generateMetricChart(deployDir, metricResponse)

	}

	os.Exit(0)
}
