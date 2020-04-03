package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"time"
)

type TeamStatistics struct {
	Wus    int `json:"wus"`
	Donors []struct {
		Wus    int    `json:"wus"`
		Name   string `json:"name"`
		Rank   int    `json:"rank"`
		Credit int    `json:"credit"`
		Team   int    `json:"team"`
		ID     int    `json:"id"`
	} `json:"donors"`
	Rank       int    `json:"rank"`
	TotalTeams int    `json:"total_teams"`
	Active50   int    `json:"active_50"`
	Logo       string `json:"logo"`
	WusCert    string `json:"wus_cert"`
	CreditCert string `json:"credit_cert"`
	Last       string `json:"last"`
	Name       string `json:"name"`
	URL        string `json:"url"`
	Credit     int    `json:"credit"`
	Team       int    `json:"team"`
	Path       string `json:"path"`
}

var (
	flagTeamID = flag.Int("teamID", 0, "Team ID (/api/team/<n>)")
	flagAddr   = flag.String("listen", "[::]:8080", "Listen address")
)

func init() {
	flag.Parse()

	if *flagTeamID == 0 {
		log.Fatal("please specify -teamID")
	}
}

const (
	httpTimeout = 2 * time.Second
)

type foldingCollector struct {
	client *http.Client
	teamID int

	donorRankDesc    *prometheus.Desc
	donorCreditDesc  *prometheus.Desc
	donorWUCountDesc *prometheus.Desc

	teamRankDesc    *prometheus.Desc
	teamCreditDesc  *prometheus.Desc
	teamWUCountDesc *prometheus.Desc

	totalTeamsDesc *prometheus.Desc
}

func NewFoldingCollector(teamID int) prometheus.Collector {
	return &foldingCollector{
		client: &http.Client{Timeout: httpTimeout},
		teamID: teamID,
		donorRankDesc: prometheus.NewDesc(
			"folding_donor_rank",
			"Individual donor's rank",
			[]string{"id", "name", "team"}, nil),
		donorCreditDesc: prometheus.NewDesc(
			"folding_donor_credit",
			"Individual donor's credit",
			[]string{"id", "name", "team"}, nil),
		donorWUCountDesc: prometheus.NewDesc(
			"folding_donor_work_units_total",
			"Individual donor's completed work units",
			[]string{"id", "name", "team"}, nil),

		teamRankDesc: prometheus.NewDesc(
			"folding_team_rank",
			"Team's rank",
			[]string{"id", "name"}, nil),
		teamCreditDesc: prometheus.NewDesc(
			"folding_team_credit",
			"Team's credit",
			[]string{"id", "name"}, nil),
		teamWUCountDesc: prometheus.NewDesc(
			"folding_team_work_units_total",
			"Team's completed work units",
			[]string{"id", "name"}, nil),

		totalTeamsDesc: prometheus.NewDesc(
			"folding_teams_total",
			"Total number of teams",
			nil, nil),
	}
}

func (collector foldingCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- collector.donorRankDesc
	ch <- collector.donorCreditDesc
	ch <- collector.donorWUCountDesc

	ch <- collector.teamRankDesc
	ch <- collector.teamCreditDesc
	ch <- collector.teamWUCountDesc

	ch <- collector.totalTeamsDesc
}

func (collector foldingCollector) mustEmitMetrics(ch chan<- prometheus.Metric, response *TeamStatistics) {
	ch <- prometheus.MustNewConstMetric(collector.totalTeamsDesc, prometheus.GaugeValue,
		float64(response.TotalTeams))

	ch <- prometheus.MustNewConstMetric(collector.teamRankDesc, prometheus.GaugeValue,
		float64(response.Rank), strconv.Itoa(response.Team), response.Name)
	ch <- prometheus.MustNewConstMetric(collector.teamCreditDesc, prometheus.GaugeValue,
		float64(response.Credit), strconv.Itoa(response.Team), response.Name)
	ch <- prometheus.MustNewConstMetric(collector.teamWUCountDesc, prometheus.CounterValue,
		float64(response.Wus), strconv.Itoa(response.Team), response.Name)

	for _, donor := range response.Donors {
		ch <- prometheus.MustNewConstMetric(collector.donorRankDesc, prometheus.GaugeValue,
			float64(donor.Rank), strconv.Itoa(donor.ID), donor.Name, strconv.Itoa(donor.Team))
		ch <- prometheus.MustNewConstMetric(collector.donorCreditDesc, prometheus.GaugeValue,
			float64(donor.Credit), strconv.Itoa(donor.ID), donor.Name, strconv.Itoa(donor.Team))
		ch <- prometheus.MustNewConstMetric(collector.donorWUCountDesc, prometheus.CounterValue,
			float64(donor.Wus), strconv.Itoa(donor.ID), donor.Name, strconv.Itoa(donor.Team))
	}
}

func (collector foldingCollector) Collect(ch chan<- prometheus.Metric) {
	var (
		stats TeamStatistics
		body  []byte
		err   error
	)

	resp, err := collector.client.Get(fmt.Sprintf("https://stats.foldingathome.org/api/team/%d", collector.teamID))
	if err != nil {
		goto error
	}
	defer resp.Body.Close()

	body, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		goto error
	}

	if err = json.Unmarshal(body, &stats); err != nil {
		goto error
	}

	collector.mustEmitMetrics(ch, &stats)
	return

error:
	ch <- prometheus.NewInvalidMetric(collector.totalTeamsDesc, err)
}

func main() {
	collector := NewFoldingCollector(*flagTeamID)
	prometheus.MustRegister(collector)
	http.Handle("/metrics", promhttp.Handler())
	panic(http.ListenAndServe(*flagAddr, nil))
}
