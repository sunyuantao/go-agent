package internal

import (
	"strconv"
	"testing"
	"time"
)

func TestTxnTrace(t *testing.T) {
	start := time.Date(2014, time.November, 28, 1, 1, 0, 0, time.UTC)
	tr := &Tracer{}
	tr.TxnTrace.Enabled = true
	tr.TxnTrace.StackTraceThreshold = 1 * time.Hour
	tr.TxnTrace.SegmentThreshold = 0

	t1 := StartSegment(tr, start.Add(1*time.Second))
	t2 := StartSegment(tr, start.Add(2*time.Second))
	EndDatastoreSegment(EndDatastoreParams{
		Tracer:             tr,
		Start:              t2,
		Now:                start.Add(3 * time.Second),
		Product:            "MySQL",
		Operation:          "SELECT",
		Collection:         "my_table",
		ParameterizedQuery: "INSERT INTO users (name, age) VALUES ($1, $2)",
		QueryParameters:    vetQueryParameters(map[string]interface{}{"zip": 1}),
		Database:           "my_db",
		Host:               "db-server-1",
		PortPathOrID:       "3306",
	})
	t3 := StartSegment(tr, start.Add(4*time.Second))
	EndExternalSegment(tr, t3, start.Add(5*time.Second), parseURL("http://example.com/zip/zap?secret=shhh"))
	EndBasicSegment(tr, t1, start.Add(6*time.Second), "t1")
	t4 := StartSegment(tr, start.Add(7*time.Second))
	t5 := StartSegment(tr, start.Add(8*time.Second))
	t6 := StartSegment(tr, start.Add(9*time.Second))
	EndBasicSegment(tr, t6, start.Add(10*time.Second), "t6")
	EndBasicSegment(tr, t5, start.Add(11*time.Second), "t5")
	t7 := StartSegment(tr, start.Add(12*time.Second))
	EndDatastoreSegment(EndDatastoreParams{
		Tracer:    tr,
		Start:     t7,
		Now:       start.Add(13 * time.Second),
		Product:   "MySQL",
		Operation: "SELECT",
		// no collection
	})
	t8 := StartSegment(tr, start.Add(14*time.Second))
	EndExternalSegment(tr, t8, start.Add(15*time.Second), nil)
	EndBasicSegment(tr, t4, start.Add(16*time.Second), "t4")

	acfg := CreateAttributeConfig(sampleAttributeConfigInput)
	attr := NewAttributes(acfg)
	attr.Agent.RequestMethod = "GET"
	AddUserAttribute(attr, "zap", 123, DestAll)

	ht := HarvestTrace{
		Start:      start,
		Duration:   20 * time.Second,
		MetricName: "WebTransaction/Go/hello",
		CleanURL:   "/url",
		Trace:      tr.TxnTrace,
		Attrs:      attr,
	}

	expect := CompactJSONString(`
[0,{},{},
	[0,20000,"ROOT",{},[[0,20000,"WebTransaction/Go/hello",{},[
		[1000,6000,"Custom/t1",{},[
			[2000,3000,"Datastore/statement/MySQL/my_table/SELECT",{
				"database_name":"my_db",
				"host":"db-server-1",
				"port_path_or_id":"3306",
				"query":"INSERT INTO users (name, age) VALUES ($1, $2)",
				"query_parameters":{"zip":1}
			},[]],
			[4000,5000,"External/example.com/all",{"uri":"http://example.com/zip/zap"},[]]
		]],
		[7000,16000,"Custom/t4",{},[
			[8000,11000,"Custom/t5",{},[
				[9000,10000,"Custom/t6",{},[]]
			]],
			[12000,13000,"Datastore/operation/MySQL/SELECT",{
				"host":"unknown",
				"port_path_or_id":"unknown",
				"query":"'SELECT' on 'unknown' using 'MySQL'"
			},[]],
			[14000,15000,"External/unknown/all",{},[]]
		]]
	]]]],
	{
		"agentAttributes":{"request.method":"GET"},
		"userAttributes":{"zap":123},
		"intrinsics":{}
	}
]`)
	js := traceDataJSON(&ht)
	if string(js) != expect {
		t.Error(string(js), expect)
	}
}

func TestTxnTraceNoSegmentsNoAttributes(t *testing.T) {
	start := time.Date(2014, time.November, 28, 1, 1, 0, 0, time.UTC)
	tr := &Tracer{}
	tr.TxnTrace.Enabled = true
	tr.TxnTrace.StackTraceThreshold = 1 * time.Hour
	tr.TxnTrace.SegmentThreshold = 0

	acfg := CreateAttributeConfig(sampleAttributeConfigInput)
	attr := NewAttributes(acfg)

	ht := HarvestTrace{
		Start:      start,
		Duration:   20 * time.Second,
		MetricName: "WebTransaction/Go/hello",
		CleanURL:   "/url",
		Trace:      tr.TxnTrace,
		Attrs:      attr,
	}

	expect := CompactJSONString(`
[0,{},{},
	[0,20000,"ROOT",{},[[0,20000,"WebTransaction/Go/hello",{},[
	]]]],
	{
		"agentAttributes":{},
		"userAttributes":{},
		"intrinsics":{}
	}
]`)
	js := traceDataJSON(&ht)
	if string(js) != expect {
		t.Error(string(js), expect)
	}
}

func TestTxnTraceSlowestNodesSaved(t *testing.T) {
	start := time.Date(2014, time.November, 28, 1, 1, 0, 0, time.UTC)
	tr := &Tracer{}
	tr.TxnTrace.Enabled = true
	tr.TxnTrace.StackTraceThreshold = 1 * time.Hour
	tr.TxnTrace.SegmentThreshold = 0
	tr.TxnTrace.maxNodes = 5

	durations := []int{5, 4, 6, 3, 7, 2, 8, 1, 9}
	now := start
	for _, d := range durations {
		s := StartSegment(tr, now)
		now = now.Add(time.Duration(d) * time.Second)
		EndBasicSegment(tr, s, now, strconv.Itoa(d))
	}

	acfg := CreateAttributeConfig(sampleAttributeConfigInput)
	attr := NewAttributes(acfg)

	ht := HarvestTrace{
		Start:      start,
		Duration:   123 * time.Second,
		MetricName: "WebTransaction/Go/hello",
		CleanURL:   "/url",
		Trace:      tr.TxnTrace,
		Attrs:      attr,
	}

	expect := CompactJSONString(`
[0,{},{},
	[0,123000,"ROOT",{},[[0,123000,"WebTransaction/Go/hello",{},[
		[0,5000,"Custom/5",{},[]],
		[9000,15000,"Custom/6",{},[]],
		[18000,25000,"Custom/7",{},[]],
		[27000,35000,"Custom/8",{},[]],
		[36000,45000,"Custom/9",{},[]]
	]]]],
	{
		"agentAttributes":{},
		"userAttributes":{},
		"intrinsics":{}
	}
]`)
	js := traceDataJSON(&ht)
	if string(js) != expect {
		t.Error(string(js), expect)
	}
}

func TestTxnTraceSegmentThreshold(t *testing.T) {
	start := time.Date(2014, time.November, 28, 1, 1, 0, 0, time.UTC)
	tr := &Tracer{}
	tr.TxnTrace.Enabled = true
	tr.TxnTrace.StackTraceThreshold = 1 * time.Hour
	tr.TxnTrace.SegmentThreshold = 7 * time.Second
	tr.TxnTrace.maxNodes = 5

	durations := []int{5, 4, 6, 3, 7, 2, 8, 1, 9}
	now := start
	for _, d := range durations {
		s := StartSegment(tr, now)
		now = now.Add(time.Duration(d) * time.Second)
		EndBasicSegment(tr, s, now, strconv.Itoa(d))
	}

	acfg := CreateAttributeConfig(sampleAttributeConfigInput)
	attr := NewAttributes(acfg)

	ht := HarvestTrace{
		Start:      start,
		Duration:   123 * time.Second,
		MetricName: "WebTransaction/Go/hello",
		CleanURL:   "/url",
		Trace:      tr.TxnTrace,
		Attrs:      attr,
	}

	expect := CompactJSONString(`
[0,{},{},
	[0,123000,"ROOT",{},[[0,123000,"WebTransaction/Go/hello",{},[
		[18000,25000,"Custom/7",{},[]],
		[27000,35000,"Custom/8",{},[]],
		[36000,45000,"Custom/9",{},[]]
	]]]],
	{
		"agentAttributes":{},
		"userAttributes":{},
		"intrinsics":{}
	}
]`)
	js := traceDataJSON(&ht)
	if string(js) != expect {
		t.Error(string(js), expect)
	}
}

func TestEmptyHarvestTraces(t *testing.T) {
	start := time.Date(2014, time.November, 28, 1, 1, 0, 0, time.UTC)
	ht := newHarvestTraces()
	js, err := ht.Data("12345", start)
	if nil != err || nil != js {
		t.Error(string(js), err)
	}
}

func TestLongestTraceSaved(t *testing.T) {
	start := time.Date(2014, time.November, 28, 1, 1, 0, 0, time.UTC)
	tr := &Tracer{}
	tr.TxnTrace.Enabled = true

	acfg := CreateAttributeConfig(sampleAttributeConfigInput)
	attr := NewAttributes(acfg)
	ht := newHarvestTraces()

	ht.Witness(HarvestTrace{
		Start:      start,
		Duration:   3 * time.Second,
		MetricName: "WebTransaction/Go/3",
		CleanURL:   "/url/3",
		Trace:      tr.TxnTrace,
		Attrs:      attr,
	})
	ht.Witness(HarvestTrace{
		Start:      start,
		Duration:   5 * time.Second,
		MetricName: "WebTransaction/Go/5",
		CleanURL:   "/url/5",
		Trace:      tr.TxnTrace,
		Attrs:      attr,
	})
	ht.Witness(HarvestTrace{
		Start:      start,
		Duration:   4 * time.Second,
		MetricName: "WebTransaction/Go/4",
		CleanURL:   "/url/4",
		Trace:      tr.TxnTrace,
		Attrs:      attr,
	})

	expect := CompactJSONString(`
[
	"12345",
	[
		[
			1417136460000000,5000,"WebTransaction/Go/5","/url/5",
			[
				0,{},{},
				[0,5000,"ROOT",{},
					[[0,5000,"WebTransaction/Go/5",{},[]]]
				],
				{
					"agentAttributes":{},
					"userAttributes":{},
					"intrinsics":{}
				}
			],
			"",null,false,null,""
		]
	]
]`)
	js, err := ht.Data("12345", start)
	if nil != err || string(js) != expect {
		t.Error(err, string(js), expect)
	}
}

func TestTxnTraceStackTraceThreshold(t *testing.T) {
	start := time.Date(2014, time.November, 28, 1, 1, 0, 0, time.UTC)
	tr := &Tracer{}
	tr.TxnTrace.Enabled = true
	tr.TxnTrace.StackTraceThreshold = 2 * time.Second
	tr.TxnTrace.SegmentThreshold = 0
	tr.TxnTrace.maxNodes = 5

	// below stack trace threshold
	t1 := StartSegment(tr, start.Add(1*time.Second))
	EndBasicSegment(tr, t1, start.Add(2*time.Second), "t1")

	// not above stack trace threshold w/out params
	t2 := StartSegment(tr, start.Add(2*time.Second))
	EndDatastoreSegment(EndDatastoreParams{
		Tracer:     tr,
		Start:      t2,
		Now:        start.Add(4 * time.Second),
		Product:    "MySQL",
		Collection: "my_table",
		Operation:  "SELECT",
	})

	// node above stack trace threshold w/ params
	t3 := StartSegment(tr, start.Add(4*time.Second))
	EndExternalSegment(tr, t3, start.Add(6*time.Second), parseURL("http://example.com/zip/zap?secret=shhh"))

	p := tr.TxnTrace.nodes[0].params
	if nil != p {
		t.Error(p)
	}
	p = tr.TxnTrace.nodes[1].params
	if nil == p || nil == p.StackTrace || "" != p.CleanURL {
		t.Error(p)
	}
	p = tr.TxnTrace.nodes[2].params
	if nil == p || nil == p.StackTrace || "http://example.com/zip/zap" != p.CleanURL {
		t.Error(p)
	}
}
