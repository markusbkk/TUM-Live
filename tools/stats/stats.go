package stats

import (
	"context"
	"fmt"
	influxdb2 "github.com/influxdata/influxdb-client-go"
	"github.com/influxdata/influxdb-client-go/api"
	"github.com/joschahenningsen/TUM-Live/model"
	"time"
)

type Stats struct {
	bucket    string
	client    influxdb2.Client
	liveStats api.WriteAPI
	query     api.QueryAPI
}

var Client *Stats

func InitStats(client influxdb2.Client) {
	bucket := "live_stats"
	Client = &Stats{
		bucket:    bucket,
		client:    client,
		liveStats: client.WriteAPI("rbg", bucket),
		query:     client.QueryAPI("rbg"),
	}
}

func (s *Stats) AddStreamStat(courseId string, stat model.Stat) {
	p := influxdb2.NewPoint("viewers",
		map[string]string{"live": fmt.Sprintf("%v", stat.Live), "stream": fmt.Sprintf("%d", stat.StreamID), "course": courseId},
		map[string]interface{}{"num": stat.Viewers},
		time.Now())
	s.liveStats.WritePoint(p)
	s.liveStats.Flush()
}

func (s *Stats) AddStreamVODStat(courseId string, streamId string) {
	viewerCount := 1

	result, err := s.query.Query(context.Background(), `from(bucket:"`+s.bucket+`") |> range(start: 0) |> filter(fn: (r) => r.stream == "`+streamId+`" and r.course == "`+courseId+`" and r.live == "false") |> last()`)
	if err == nil && result.Record() != nil {
		num := result.Record().ValueByKey("num").(int)
		viewerCount += num
	}

	p := influxdb2.NewPoint("viewers",
		map[string]string{"live": "false", "stream": streamId, "course": courseId},
		map[string]interface{}{"num": viewerCount},
		time.Now())

	s.liveStats.WritePoint(p)
	s.liveStats.Flush()
}
