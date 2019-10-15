package calendar

import (
	"testing"
	"time"

	libdate "github.com/rickb777/date"
	"github.com/stretchr/testify/assert"
)

func TestParseRemind(t *testing.T) {
	tests := []struct {
		t      time.Time
		remind string
		result time.Time
		hasErr bool
	}{
		{
			t:      newTimeYMDHM(2019, 8, 29, 0, 0),
			remind: "",
			hasErr: false,
			result: time.Time{},
		},

		{
			t:      newTimeYMDHM(2019, 8, 29, 0, 0),
			remind: "0",
			hasErr: false,
			result: newTimeYMDHM(2019, 8, 29, 0, 0),
		},

		{
			t:      newTimeYMDHM(2019, 8, 29, 2, 0),
			remind: "60", // 提前 60 min
			hasErr: false,
			result: newTimeYMDHM(2019, 8, 29, 1, 0),
		},

		{
			t:      newTimeYMDHM(2019, 8, 29, 2, 0),
			remind: "abc", // 错误的值
			hasErr: true,
			result: time.Time{},
		},

		{
			t:      newTimeYMDHM(2019, 8, 29, 0, 0),
			remind: "0;08:00", // 当天的08:00
			hasErr: false,
			result: newTimeYMDHM(2019, 8, 29, 8, 0),
		},

		{
			t:      newTimeYMDHM(2019, 8, 29, 0, 0),
			remind: "1;08:00", // 提前一天
			hasErr: false,
			result: newTimeYMDHM(2019, 8, 28, 8, 0),
		},

		{
			t:      newTimeYMDHM(2019, 8, 29, 0, 0),
			remind: "0;24:00", // 错误的时间
			hasErr: true,
			result: time.Time{},
		},

		{
			t:      newTimeYMDHM(2019, 8, 29, 0, 0),
			remind: "0;00:60", // 错误的时间
			hasErr: true,
			result: time.Time{},
		},
	}
	for idx, test := range tests {
		rt, err := parseRemind(test.t, test.remind)
		assert.Equal(t, test.result, rt, "test idx: %d", idx)
		if test.hasErr {
			assert.NotNil(t, err, "test idx: %d", idx)
		} else {
			assert.Nil(t, err, "test idx: %d", idx)
		}
	}
}

func TestGetRemindAdvanceDays(t *testing.T) {
	n, err := getRemindAdvanceDays("0;09:00")
	assert.Nil(t, err)
	assert.Equal(t, 0, n)
	n, err = getRemindAdvanceDays("1;09:00")
	assert.Nil(t, err)
	assert.Equal(t, 1, n)

	n, err = getRemindAdvanceDays("0")
	assert.Nil(t, err)
	assert.Equal(t, 0, n)

	n, err = getRemindAdvanceDays("2880")
	assert.Nil(t, err)
	assert.Equal(t, 2, n)
}

func TestTimeRangeContains(t *testing.T) {
	r := getTimeRange(newTimeYMDHM(2019, 1, 1, 0, 0),
		newTimeYMDHM(2019, 1, 1, 2, 0))
	r1 := getTimeRange(newTimeYMDHM(2019, 1, 1, 0, 0),
		newTimeYMDHM(2019, 1, 1, 1, 0))
	assert.True(t, r.contains(r1))
	assert.False(t, r1.contains(r))

	r = getTimeRange(newTimeYMDHM(2019, 1, 1, 1, 0),
		newTimeYMDHM(2019, 1, 1, 2, 0))
	r1 = getTimeRange(newTimeYMDHM(2019, 1, 1, 0, 0),
		newTimeYMDHM(2019, 1, 1, 1, 0))
	assert.False(t, r.contains(r1))
	assert.False(t, r1.contains(r))

	r = getTimeRange(newTimeYMDHM(2019, 1, 1, 1, 0),
		newTimeYMDHM(2019, 1, 1, 2, 0))
	r1 = getTimeRange(newTimeYMDHM(2019, 1, 1, 0, 0),
		newTimeYMDHM(2019, 1, 1, 3, 0))
	assert.False(t, r.contains(r1))
	assert.True(t, r1.contains(r))
}

func TestTimeRangeOverlap(t *testing.T) {
	r := getTimeRange(newTimeYMDHM(2019, 1, 1, 0, 0),
		newTimeYMDHM(2019, 1, 1, 2, 0))
	r1 := getTimeRange(newTimeYMDHM(2019, 1, 1, 0, 0),
		newTimeYMDHM(2019, 1, 1, 1, 0))
	assert.True(t, r.overlap(r1))
	assert.True(t, r1.overlap(r))

	r = getTimeRange(newTimeYMDHM(2019, 1, 1, 0, 0),
		newTimeYMDHM(2019, 1, 1, 1, 0))
	r1 = getTimeRange(newTimeYMDHM(2019, 1, 1, 2, 0),
		newTimeYMDHM(2019, 1, 1, 3, 0))
	assert.False(t, r.overlap(r1))
	assert.False(t, r1.overlap(r))

	r = getTimeRange(newTimeYMDHM(2019, 1, 1, 0, 0),
		newTimeYMDHMS(2019, 1, 1, 23, 59, 59))
	r1 = getTimeRange(newTimeYMDHM(2019, 1, 2, 0, 0),
		newTimeYMDHMS(2019, 1, 2, 23, 59, 59))
	assert.False(t, r.overlap(r1))
	assert.False(t, r1.overlap(r))
}

func TestBetween(t *testing.T) {
	// 单日任务，无重复
	job := &Job{
		Start: newTimeYMDHM(2019, 9, 1, 9, 0),
		End:   newTimeYMDHM(2019, 9, 1, 10, 0),
	}
	startDate := libdate.New(2019, 9, 1)
	endDate := libdate.New(2019, 9, 10)
	jobTimes, err := job.between(startDate, endDate)
	assert.Nil(t, err)
	assert.Len(t, jobTimes, 1)
	assert.Equal(t, jobTime{start: newTimeYMDHM(2019, 9, 1, 9, 0)}, jobTimes[0])

	startDate = libdate.New(2019, 8, 1)
	endDate = libdate.New(2019, 8, 31)
	jobTimes, err = job.between(startDate, endDate)
	assert.Nil(t, err)
	assert.Len(t, jobTimes, 0)

	startDate = libdate.New(2019, 9, 2)
	endDate = libdate.New(2019, 9, 31)
	jobTimes, err = job.between(startDate, endDate)
	assert.Nil(t, err)
	assert.Len(t, jobTimes, 0)

	// 单日任务，重复：每天
	job = &Job{
		Start: newTimeYMDHM(2019, 9, 1, 9, 0),
		End:   newTimeYMDHM(2019, 9, 1, 10, 0),
		RRule: "FREQ=DAILY",
	}
	startDate = libdate.New(2019, 9, 1)
	endDate = libdate.New(2019, 9, 10)
	jobTimes, err = job.between(startDate, endDate)
	assert.Nil(t, err)
	assert.Equal(t, len(jobTimes), 10)
	assert.Equal(t, jobTimes[0], jobTime{start: newTimeYMDHM(2019, 9, 1, 9, 0)})
	assert.Equal(t, jobTimes[1], jobTime{start: newTimeYMDHM(2019, 9, 2, 9, 0), recurID: 1})
	assert.Equal(t, jobTimes[9], jobTime{start: newTimeYMDHM(2019, 9, 10, 9, 0), recurID: 9})

	// 多日任务，10日， 无重复
	job = &Job{
		Start: newTimeYMDHM(2019, 9, 1, 9, 0),
		End:   newTimeYMDHM(2019, 9, 10, 9, 0),
	}
	startDate = libdate.New(2019, 9, 1)
	endDate = libdate.New(2019, 9, 12)
	jobTimes, err = job.between(startDate, endDate)
	assert.Nil(t, err)
	assert.Len(t, jobTimes, 1)
	assert.Equal(t, jobTimes[0], jobTime{start: newTimeYMDHM(2019, 9, 1, 9, 0)})

	startDate = libdate.New(2019, 9, 5)
	endDate = libdate.New(2019, 9, 12)
	jobTimes, err = job.between(startDate, endDate)
	assert.Nil(t, err)
	assert.Len(t, jobTimes, 1)
	assert.Equal(t, jobTimes[0], jobTime{start: newTimeYMDHM(2019, 9, 1, 9, 0)})

	startDate = libdate.New(2019, 8, 1)
	endDate = libdate.New(2019, 8, 31)
	jobTimes, err = job.between(startDate, endDate)
	assert.Nil(t, err)
	assert.Len(t, jobTimes, 0)

	startDate = libdate.New(2019, 9, 11)
	endDate = libdate.New(2019, 9, 30)
	jobTimes, err = job.between(startDate, endDate)
	assert.Nil(t, err)
	assert.Len(t, jobTimes, 0)
}
