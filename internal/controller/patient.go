package controller

import (
	"fmt"
	"time"

	"pmain2/internal/consts"
	"pmain2/internal/database"
	"pmain2/internal/models"
	"pmain2/internal/types"
	"pmain2/pkg/cache"
	"pmain2/pkg/utils"
)

var (
	cachePat = cache.CreateCache(time.Minute, time.Minute)
)

type patient struct{}

func initPatientController() *patient {
	return &patient{}
}

func (p *patient) FindByFio(lname, fname, sname string) (*[]models.Patient, error) {
	cacheName := lname + " " + fname + " " + sname

	item, ok := cachePat.Get(cacheName)
	if ok {
		res := item.(*[]models.Patient)
		return res, nil
	}

	model := models.Model.Patient
	lname, _ = utils.ToWin1251(lname)
	fname, _ = utils.ToWin1251(fname)
	sname, _ = utils.ToWin1251(sname)
	data, err := model.FindByFIO(lname, fname, sname)
	if err != nil {
		return nil, err
	}

	cachePat.Set(cacheName, data, 0)
	return data, nil
}

func (p *patient) FindById(id int) (*models.Patient, error) {

	conn, err := database.Connect()
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	model := models.Init(conn.DB).Patient
	data, err := model.Get(id)
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (p *patient) FindUchet(id int) (*[]models.FindUchetS, error) {
	cacheName := fmt.Sprintf("find_uchet_%v", id)
	item, ok := cache.AppCache.Get(cacheName)
	if ok {
		return item.(*[]models.FindUchetS), nil
	}
	model := models.Model.Patient
	data, err := model.FindUchet(id)
	if err != nil {
		ERROR.Println(err.Error())
		return nil, err
	}
	cache.AppCache.Set(cacheName, data, 0)
	return data, nil
}

func (p *patient) HistoryVisits(id int, isCache bool) (*[]models.HistoryVisit, error) {
	cacheName := fmt.Sprintf("disp_history_Visit_%v", id)
	item, ok := cache.AppCache.Get(cacheName)
	if ok && isCache {
		return item.(*[]models.HistoryVisit), nil
	}
	model := models.Model.Patient
	data, err := model.HistoryVisits(id)
	if err != nil {
		ERROR.Println(err.Error())
		return nil, err
	}
	cache.AppCache.Set(cacheName, data, 0)
	return data, nil
}

func (p *patient) HistoryHospital(id int) (*[]models.HistoryHospital, error) {
	cacheName := fmt.Sprintf("disp_history_hospital_%v", id)
	item, ok := cache.AppCache.Get(cacheName)
	if ok {
		return item.(*[]models.HistoryHospital), nil
	}
	model := models.Model.Patient
	data, err := model.HistoryHospital(id)
	if err != nil {
		ERROR.Println(err.Error())
		return nil, err
	}
	cache.AppCache.Set(cacheName, data, 0)
	return data, nil
}

func (p *patient) NewVisit(visit *types.NewVisit) (int, error) {
	fmt.Println(*visit)
	visit.Normalize()
	model := models.Model.Patient
	lastUchet, err := model.FindLastUchet(visit.PatientId)
	if err != nil {
		return 100, err
	}

	//-проверить что пациент не мертв или это работа с документами
	if (lastUchet != nil && lastUchet.Reason == consts.REAS_DEAD) && visit.Visit&consts.VISIT_WORK_WITH_DOCUMENTS == 0 {
		return 101, nil
	}

	//-в этот день не было посещений
	isVisisted, err := model.IsVisited(visit)
	if err != nil {
		return 102, err
	}

	if isVisisted {
		return 202, nil
	}

	conn, err := database.Connect()
	if err != nil {
		return 20, err
	}
	tx, err := conn.DB.Begin()
	if err != nil {
		return 21, err
	}

	//Обрезаем до 10, т.к. в посещениях длина диагноза 10
	visit.Diagnose = visit.Diagnose[0:10]

	model = models.Model.Patient
	_, err = model.NewVisit(*visit, tx)
	if err != nil {
		tx.Rollback()
		return 200, err
	}
	if visit.SRC >= 0 {
		_, err = model.NewSRC(&types.NewSRC{
			PatientId: visit.PatientId,
			DateAdd:   visit.Date,
			DockId:    visit.DockId,
			Unit:      visit.Unit,
			Zakl:      visit.SRC,
		}, tx)
		if err != nil {
			tx.Rollback()
			return 201, err
		}
	}

	err = tx.Commit()
	if err != nil {
		return 22, err
	}

	return 0, nil
}

func (p *patient) NewProf(visit *types.NewProf) (int, error) {
	fmt.Println(*visit)
	visit.Normalize()
	model := models.Model.Patient

	conn, err := database.Connect()
	if err != nil {
		return 20, err
	}
	tx, err := conn.DB.Begin()
	if err != nil {
		return 21, err
	}

	if visit.Count == 0 {
		return 203, nil
	}

	model = models.Model.Patient
	for i := 0; i < visit.Count; i++ {
		_, err = model.NewProf(*visit, tx)
		if err != nil {
			tx.Rollback()
			return 200, err
		}
	}

	err = tx.Commit()
	if err != nil {
		return 22, err
	}

	return 0, nil
}
