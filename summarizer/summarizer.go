package summarizer

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"regexp"
	"strconv"
	"time"
)

type Summarizer struct {
	forwards  []chan rawData
	durations []time.Duration
	reqs      []chan int
	resps     []chan *Archive
}

type rawData struct {
	timestamp time.Time
	val       float64
}

type Archive struct {
	interval time.Duration
	count    int
	vals     []ArchivedData
}

type ArchivedData struct {
	Timestamp time.Time `json:"timestamp"`
	Min       float64   `json:"min"`
	Max       float64   `json:"max"`
	Ave       float64   `json:"ave"`
	Count     int       `json:"count"`
}

func New() *Summarizer {
	summ := Summarizer{}
	return &summ
}

func (summ *Summarizer) Send(val float64) {
	for _, ch := range summ.forwards {
		ch <- rawData{time.Now(), val}
	}
}

func (summ *Summarizer) Handler(w http.ResponseWriter, r *http.Request) {
	re := regexp.MustCompile("/(\\d+)\\.json$")
	matches := re.FindStringSubmatch(r.URL.Path)
	var sec int
	if matches == nil {
		sec = 1800
	} else {
		sec, _ = strconv.Atoi(matches[1])
	}
	// duration秒のデータを返す
	duration := time.Duration(sec) * time.Second
	var target_archive int = -1
	for i, v := range summ.durations {
		// duration秒が返せるarchiveを探す
		if v >= duration {
			target_archive = i
			break
		}
	}
	if target_archive == -1 {
		target_archive = len(summ.durations) - 1
	}

	summ.reqs[target_archive] <- 0
	smrzd := <-summ.resps[target_archive]

	latest := smrzd.latest(duration)

	if target_archive > 0 && len(latest) > 0 {
		summ.reqs[target_archive-1] <- 0
		smrzd := <-summ.resps[target_archive-1]
		last_index := len(latest) - 1
		replacement := smrzd.since(latest[last_index].Timestamp)

		if last_index == 0 {
			latest = replacement
		} else {
			tmp := make([]ArchivedData, last_index+len(replacement))
			copy(tmp, latest[:last_index])
			copy(tmp[last_index:], replacement)
			latest = tmp
		}
	}

	b, err := json.Marshal(latest)
	if err == nil {
		fmt.Fprintf(w, string(b))
	}
}

func (summ *Summarizer) AddArchive(interval time.Duration, count int) {
	smrzd := Archive{interval, count, make([]ArchivedData, 0, count*2+1)}
	smrzd.vals = append(smrzd.vals, ArchivedData{time.Now().Truncate(interval), math.MaxFloat64, -math.MaxFloat64, 0, 0})

	forward_ch := make(chan rawData)
	req_ch := make(chan int)
	res_ch := make(chan *Archive)

	go func() {
		for {
			select {
			case result := <-forward_ch:
				smrzd.append(result)
			case <-req_ch:
				res_ch <- &smrzd
				/*
					b, err := json.Marshal(smrzd.vals)
					if err != nil {
						res_ch <- "internal error!"
					} else {
						res_ch <- string(b)
					}
				*/
			}
		}
	}()

	summ.forwards = append(summ.forwards, forward_ch)
	summ.durations = append(summ.durations, interval*time.Duration(count))
	summ.reqs = append(summ.reqs, req_ch)
	summ.resps = append(summ.resps, res_ch)
}

func (smrzd *Archive) append(el rawData) {
	trunc := el.timestamp.Truncate(smrzd.interval)

	last_el := smrzd.vals[len(smrzd.vals)-1]
	if last_el.Timestamp == trunc {
		if last_el.Max < el.val {
			last_el.Max = el.val
		}
		if last_el.Min > el.val {
			last_el.Min = el.val
		}
		last_el.Ave = (last_el.Ave*float64(last_el.Count) + el.val) / float64(last_el.Count+1)
		last_el.Count++
		// アトミックに書き換える
		smrzd.vals[len(smrzd.vals)-1] = last_el
	} else {
		smrzd.vals = append(smrzd.vals, ArchivedData{trunc, el.val, el.val, el.val, 1})
		if len(smrzd.vals) > smrzd.count*2 {
			tmp := make([]ArchivedData, smrzd.count+1, smrzd.count*2+1)
			copy(tmp, smrzd.vals[(len(smrzd.vals)-smrzd.count-1):len(smrzd.vals)])
			smrzd.vals = tmp
		}
	}
}

func (smrzd *Archive) latest(duration time.Duration) []ArchivedData {
	last_index := len(smrzd.vals) - 1
	if last_index <= 0 {
		return smrzd.vals
	}
	last_ts := smrzd.vals[last_index].Timestamp
	min_ts := last_ts.Add(-duration)
	// 最大で(duration/smrzd.interval)個を返す
	min_index := last_index - int(duration/smrzd.interval)
	if min_index < 0 {
		min_index = 0
	}
	for i := min_index; i <= last_index; i++ {
		// min_tsより後の時刻のデータを返す
		if smrzd.vals[i].Timestamp.After(min_ts) {
			return smrzd.vals[i:]
		}
	}
	return nil
}

func (smrzd *Archive) since(ts time.Time) []ArchivedData {
	last_index := len(smrzd.vals) - 1
	if last_index < 0 {
		return nil
	}
	last_ts := smrzd.vals[last_index].Timestamp
	if last_ts.Before(ts) {
		return nil
	}
	// 最大で(last_ts-ts)/smrzd.interval個のデータを返す
	min_index := last_index - int(last_ts.Sub(ts)/smrzd.interval)
	if min_index < 0 {
		min_index = 0
	}
	for i := min_index; i <= last_index; i++ {
		// tsと同一時刻または後の時刻のデータを返す
		if !smrzd.vals[i].Timestamp.Before(ts) {
			return smrzd.vals[i:]
		}
	}
	return nil
}

/*
func (el rawData) MarshalJSON() ([]byte, error) {
	val, err := json.Marshal(el.val)
	if err != nil {
		return nil, err
	} else {
		return []byte(`["` + el.timestamp.Format("2006-01-02 15:04:05") + `", ` + string(val) + `]`), nil
	}
}
*/

/*
func (el ArchivedData) MarshalJSON() ([]byte, error) {
	log.Println("smel marshal")
	return json.Marshal(el)
}
*/

/*
func (ts archivedTimestamp) MarshalJSON() ([]byte, error) {
	log.Println("ts json")
	return []byte(`"` + time.Time(ts).Format("2006-01-02 15:04:05") + `"`), nil
}

func (smrzd *Archive) MarshalJSON() ([]byte, error) {
	json, err := json.Marshal(smrzd.vals)
	if err != nil {
		return nil, err
	} else {
		return json, nil
	}
}

func (ts archivedTimestamp) MarshalJSON() ([]byte, error) {
	log.Println("ts json")
	return []byte(`"` + ts.Format("2006-01-02 15:04:05") + `"`), nil
}
*/
