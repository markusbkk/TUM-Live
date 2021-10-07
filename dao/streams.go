package dao

import (
	"TUM-Live/model"
	"context"
	"fmt"
	"gorm.io/gorm"
	"strconv"
	"time"
)

func GetDueStreamsForWorkers() []model.Stream {
	var res []model.Stream
	DB.Model(&model.Stream{}).
		Where("lecture_hall_id IS NOT NULL AND start BETWEEN ? AND ? AND live_now = false AND recording = false", time.Now(), time.Now().Add(time.Minute*10)).
		Scan(&res)
	return res
}

func GetDuePremieresForWorkers() []model.Stream {
	var res []model.Stream
	DB.Preload("Files").
		Find(&res, "premiere AND start BETWEEN ? AND ? AND live_now = false AND recording = false", time.Now().Add(time.Minute*-10), time.Now().Add(time.Second*5))
	return res
}

func GetStreamByKey(ctx context.Context, key string) (stream model.Stream, err error) {
	var res model.Stream
	err = DB.First(&res, "stream_key = ?", key).Error
	return res, err
}

func DeleteUnit(id uint) {
	defer Cache.Clear()
	DB.Delete(&model.StreamUnit{}, id)
}

func GetUnitByID(id string) (model.StreamUnit, error) {
	var unit model.StreamUnit
	err := DB.First(&unit, "id = ?", id).Error
	return unit, err
}

func GetStreamByTumOnlineID(ctx context.Context, id uint) (stream model.Stream, err error) {
	var res model.Stream
	err = DB.Preload("Chats").First(&res, "tum_online_event_id = ?", id).Error
	if err != nil {
		return res, err
	}
	return res, nil
}

func GetStreamByID(ctx context.Context, id string) (stream model.Stream, err error) {
	if cached, found := Cache.Get(fmt.Sprintf("streambyid%v", id)); found {
		return cached.(model.Stream), nil
	}
	var res model.Stream
	err = DB.Preload("Silences").Preload("Chats").Preload("Units", func(db *gorm.DB) *gorm.DB {
		return db.Order("unit_start asc")
	}).First(&res, "id = ?", id).Error
	if err != nil {
		fmt.Printf("error getting stream by id: %v\n", err)
		return res, err
	}
	Cache.SetWithTTL(fmt.Sprintf("streambyid%v", id), res, 1, time.Second*10)
	return res, nil
}

func DeleteStreamsWithTumID(ids []uint) {
	// transaction for performance
	_ = DB.Transaction(func(tx *gorm.DB) error {
		for i := range ids {
			tx.Where("tum_online_event_id = ?", ids[i]).Delete(&model.Stream{})
		}
		return nil
	})
}

//AddVodView Adds a stat entry to the database or increases the one existing for this hour
func AddVodView(id string) error {
	intId, err := strconv.Atoi(id)
	if err != nil {
		return err
	}
	err = DB.Transaction(func(tx *gorm.DB) error {
		t := time.Now()
		tFrom := time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), 0, 0, 0, time.Local)
		tUntil := tFrom.Add(time.Hour)
		var stat *model.Stat
		err := DB.First(&stat, "live = 0 AND time BETWEEN ? and ?", tFrom, tUntil).Error
		if err != nil { // first view this hour, create
			stat := model.Stat{
				Time:     tFrom,
				StreamID: uint(intId),
				Viewers:  1,
				Live:     false,
			}
			err = tx.Create(&stat).Error
			return err
		} else {
			stat.Viewers += 1
			err = tx.Save(&stat).Error
			return err
		}
	})
	return err
}

func UpdateStream(stream model.Stream) error {
	defer Cache.Clear()
	err := DB.Model(&stream).Updates(map[string]interface{}{
		"name":        stream.Name,
		"description": stream.Description,
		"start":       stream.Start,
		"end":         stream.End}).Error
	return err
}

//GetAllStreams returns all streams of the server
func GetAllStreams() ([]model.Stream, error) {
	var res []model.Stream
	err := DB.Find(&res).Error
	return res, err
}

func GetCurrentLive(ctx context.Context) (currentLive []model.Stream, err error) {
	if streams, found := Cache.Get("AllCurrentlyLiveStreams"); found {
		return streams.([]model.Stream), nil
	}
	var streams []model.Stream
	if err := DB.Find(&streams, "live_now = ?", true).Error; err != nil {
		return nil, err
	}
	Cache.SetWithTTL("AllCurrentlyLiveStreams", streams, 1, time.Minute)
	return streams, err
}

func DeleteStream(streamID string) {
	DB.Where("id = ?", streamID).Delete(&model.Stream{})
	Cache.Clear()
}

func UpdateSilences(silences []model.Silence, streamID string) error {
	DB.Delete(&model.Silence{}, "stream_id = ?", streamID)
	return DB.Save(&silences).Error
}

func UpdateStreamFullAssoc(vod *model.Stream) error {
	defer Cache.Clear()
	err := DB.Session(&gorm.Session{FullSaveAssociations: true}).Updates(&vod).Error
	return err
}

func SetStreamNotLiveById(streamID uint) error {
	defer Cache.Clear()
	return DB.Exec("UPDATE `streams` SET `live_now`='0' WHERE id = ?", streamID).Error
}

func SavePauseState(streamid uint, paused bool) error {
	defer Cache.Clear()
	return DB.Model(model.Stream{}).Where("id = ?", streamid).Updates(map[string]interface{}{"Paused":paused}).Error
}

func SaveStream(vod *model.Stream) error {
	defer Cache.Clear()
	err := DB.Model(&vod).Updates(model.Stream{
		Name:            vod.Name,
		Description:     vod.Description,
		CourseID:        vod.CourseID,
		Start:           vod.Start,
		End:             vod.End,
		RoomName:        vod.RoomName,
		RoomCode:        vod.RoomCode,
		EventTypeName:   vod.EventTypeName,
		PlaylistUrl:     vod.PlaylistUrl,
		PlaylistUrlPRES: vod.PlaylistUrlPRES,
		PlaylistUrlCAM:  vod.PlaylistUrlCAM,
		FilePath:        vod.FilePath,
		LiveNow:         vod.LiveNow,
		Recording:       vod.Recording,
		Chats:           vod.Chats,
		Stats:           vod.Stats,
		Units:           vod.Units,
		VodViews:        vod.VodViews,
		StartOffset:     vod.StartOffset,
		EndOffset:       vod.EndOffset,
		Silences:        vod.Silences,
		Files:           vod.Files,
		Paused:          vod.Paused,
	}).Error
	return err
}
