package calendar

import (
	dbus "pkg.deepin.io/lib/dbus1"
	"pkg.deepin.io/lib/dbusutil"
)

const (
	dbusServiceName = "com.deepin.daemon.Calendar"
	dbusPath        = "/com/deepin/daemon/Calendar/Scheduler"
	dbusInterface   = "com.deepin.daemon.Calendar.Scheduler"
)

func (s *Scheduler) GetJob(id int64) (string, *dbus.Error) {
	job, err := s.getJob(uint(id))
	if err != nil {
		return "", dbusutil.ToError(err)
	}
	result, err := toJson(job)
	return result, dbusutil.ToError(err)
}

func (s *Scheduler) DeleteJob(id int64) *dbus.Error {
	err := s.deleteJob(uint(id))
	if err == nil {
		s.notifyJobsChange()
	}
	return dbusutil.ToError(err)
}

func (s *Scheduler) UpdateJob(jobStr string) *dbus.Error {
	var jj JobJSON
	err := fromJson(jobStr, &jj)
	if err != nil {
		return dbusutil.ToError(err)
	}

	job, err := jj.toJob()
	if err != nil {
		return dbusutil.ToError(err)
	}
	err = s.updateJob(job)
	if err == nil {
		s.notifyJobsChange()
	}
	return dbusutil.ToError(err)
}

func (s *Scheduler) CreateJob(jobStr string) (int64, *dbus.Error) {
	var jj JobJSON
	err := fromJson(jobStr, &jj)
	if err != nil {
		return 0, dbusutil.ToError(err)
	}

	job, err := jj.toJob()
	if err != nil {
		return 0, dbusutil.ToError(err)
	}
	err = s.createJob(job)
	if err != nil {
		return 0, dbusutil.ToError(err)
	}
	s.notifyJobsChange()
	return int64(job.ID), nil
}

func (s *Scheduler) GetTypes() (string, *dbus.Error) {
	types, err := s.getTypes()
	if err != nil {
		return "", dbusutil.ToError(err)
	}
	result, err := toJson(types)
	return result, dbusutil.ToError(err)
}

func (s *Scheduler) GetType(id int64) (string, *dbus.Error) {
	t, err := s.getType(uint(id))
	if err != nil {
		return "", dbusutil.ToError(err)
	}
	result, err := toJson(t)
	return result, dbusutil.ToError(err)
}

func (s *Scheduler) DeleteType(id int64) *dbus.Error {
	err := s.deleteType(uint(id))
	return dbusutil.ToError(err)
}

func (s *Scheduler) CreateType(typeStr string) (int64, *dbus.Error) {
	var jobType JobTypeJSON
	err := fromJson(typeStr, &jobType)
	if err != nil {
		return 0, dbusutil.ToError(err)
	}

	jt := jobType.toJobType()
	err = s.createType(jt)
	if err != nil {
		return 0, dbusutil.ToError(err)
	}
	return int64(jt.ID), nil
}

func (s *Scheduler) UpdateType(typeStr string) *dbus.Error {
	var jobType JobTypeJSON
	err := fromJson(typeStr, &jobType)
	if err != nil {
		return dbusutil.ToError(err)
	}

	jt := jobType.toJobType()
	err = s.updateType(jt)
	return dbusutil.ToError(err)
}
