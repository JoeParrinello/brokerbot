package statuszlib

import (
	"html/template"
	"log"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/JoeParrinello/brokerbot/cryptolib"
)

var (
	statuszMetrics *metrics
	startTime      time.Time
)

const statuszTemplate string = `<h1>BrokerBot Statusz</h1>

<p>uptime: {{.Uptime}}</p>

<p>build version: {{.BuildVersion}}</p>

<p>build time: {{.BuildTime}}</p>

<p>request count: {{.RequestCount}}</p>

<p>success count: {{.SuccessCount}}</p>

<p>error count: {{.ErrorCount}}</p>

<p>crypto price feed last updated: {{.CryptoPriceFeedLastUpdated}}</p>

<p>crypto price feed:</p>

<table>
	<tr>
		<td>Pair</td>
		<td>Price</td>
		<td>Change</td>
	</tr>
	{{ with .CryptoPriceFeed }}
		{{ range . }}
			<tr>
				<td>{{ .Pair }}</td>
				<td>{{ .Price }}</td>
				<td>{{ .Change }}</td>
			</tr>
		{{ end }}
	{{ end }}
</table>
`

type metrics struct {
	Uptime       time.Duration
	BuildVersion string
	BuildTime    string

	RequestCount int32
	SuccessCount int32
	ErrorCount   int32

	CryptoPriceFeed            []*cryptolib.PriceFeed
	CryptoPriceFeedLastUpdated time.Time
}

func init() {
	statuszMetrics = &metrics{}
	startTime = time.Now()
}

func HandleStatusz(w http.ResponseWriter, r *http.Request) {
	log.Println("Received /statusz request")
	template, err := template.New("statusz").Parse(statuszTemplate)
	if err != nil {
		log.Printf("failed to parse /status template: %v", err)
		return
	}
	log.Println("Scraping metrics for /statusz")
	statuszMetrics.Uptime = time.Since(startTime)
	statuszMetrics.CryptoPriceFeed = cryptolib.GetLatestPriceFeed()
	statuszMetrics.CryptoPriceFeedLastUpdated = cryptolib.GetLatestPriceFeedUpdateTime()

	log.Println("Writing metrics to /statusz")
	if err := template.Execute(w, statuszMetrics); err != nil {
		log.Printf("failed to write metrics to /statusz: %v", err)
		return
	}
}

func SetBuildVersion(buildVersion string) {
	statuszMetrics.BuildVersion = buildVersion
}

func SetBuildTime(buildTime string) {
	statuszMetrics.BuildTime = buildTime
}

func RecordRequest() int32 {
	return atomic.AddInt32(&statuszMetrics.RequestCount, 1)
}

func RecordSuccess() int32 {
	return atomic.AddInt32(&statuszMetrics.SuccessCount, 1)
}

func RecordError() int32 {
	return atomic.AddInt32(&statuszMetrics.ErrorCount, 1)
}
