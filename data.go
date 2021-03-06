package main 

//SELECT  DAY(time), HOUR(time), MINUTE(time), AVG(data) FROM measurements WHERE serial="0002" GROUP BY DAY(time), HOUR(time), MINUTE(time) WITH ROLLUP;
//select AVG(data), (ROUND(time / (60*10.8)) * 60 * 10.8) as rounded_time from measurements WHERE serial="0002" AND register="L1V" group by rounded_time;

import (
	"database/sql"
	_ "github.com/go-sql-driver/mysql"
	"time"
	"log"
)

type DB struct {
	*sql.DB
}

func NewOpen(dt string, c string) (DB, error) {
        db, err := sql.Open(dt,c)
        return DB{db}, err
}

func (d DB) LastMeasurement(l string, s string, r string) (p Point, err error){
	var max string
	var rq = "select MAX(time) from measurements where location=? AND serial=? AND register=?;"
	err = d.QueryRow(rq,l,s,r).Scan(&max)
	if err != nil {
		return
	}
	maxT, err := time.Parse("2006-01-02 15:04:05", max)
	if err != nil {
		return
	}
	
	var data float64
	rq = "select data from measurements where location=? AND serial=? AND register=? AND time=?;"
	err = d.QueryRow(rq,l,s,r,maxT).Scan(&data)
	if err != nil {
		return
	}
	
	p.Time = maxT
	p.Value = data	

	return		

}

func (d DB) GetMeasurements(l string, s string, r string, st time.Time, et time.Time) (m Measurement, err error) {
	//log.Println("GetMS",l,s,r,st,et)
	
	var max string
	var min string
	var rq = "select MAX(time), MIN(time) from measurements where location=? AND serial=? AND register=?;"
	err = d.QueryRow(rq,l,s,r).Scan(&max,&min)
	if err != nil {
		log.Println(err)
		return
	}
	maxT, err := time.Parse("2006-01-02 15:04:05", max)
	minT, err := time.Parse("2006-01-02 15:04:05", min)
	if err != nil {
		log.Println(err)
		return
	}	
//	if st.Before(minT) {
//		log.Println("st before")
		st = minT
//	}
//	if et.After(maxT) {
//		log.Println("et after")
		et = maxT
//	}
	
        var timedif = et.Unix() - st.Unix()
        var minSpan float32
	if (timedif > 60000) {
                var secSpan = float32(timedif/1000)
                minSpan = float32(secSpan/60)
        } else {
		minSpan = 1.0 		
	}

	//log.Println("MinSpan:",minSpan)	

	var query = "SELECT AVG(data), FROM_UNIXTIME(TRUNCATE(UNIX_TIMESTAMP(time) / (60*?), 0) * 60 * ?) as rounded_time from measurements WHERE location=? AND serial=? AND register=? AND time>=? AND time<? group by rounded_time;"
	
	//var query = "SELECT time, data FROM measurements WHERE location=? AND serial=? AND register=? AND time>=? AND time<?;"
	rows, err := d.Query(query, minSpan, minSpan, l, s, r, st, et)
	if err != nil {
		log.Println(err)
		return
	}
	defer rows.Close()

	m.Location = l
	m.Serial = s
	m.Register = r
	for rows.Next() {
		var t string
		var v float64
		var p []interface{}
		err = rows.Scan(&v, &t)
		if err != nil {
			log.Println(err)
		} else {
			ti, _ := time.Parse("2006-01-02 15:04:05", t)
			p = append(p,(ti.Unix()*1000))
			p = append(p,v)	
			m.Data = append(m.Data, p)
		}
	}

	return m,err
}

func (d DB) SetMeasurements(m Measurementx) (err error) {
	// Create tx
	tx, err := d.Begin()	
	if err != nil {
		log.Println(err)
		return
	}

	var query = "INSERT INTO measurements (location,serial,time,register,type,data) VALUES (?, ?, ?, ?, ?, ?);"
        stmt, err := tx.Prepare(query)	
	if err != nil {
		log.Println(err)
		return
	}
	
	// Defer
        defer func () {
                if err == nil {
                        log.Println("Commit")
                        tx.Commit()
                } else {
                        log.Println("RollBack")
                        tx.Rollback()
                }
		stmt.Close()
        }()
	
	for _,r := range m.KeyPairs {
		_, err = stmt.Exec(m.Location, m.Serial, m.TimeS.Local(), r.Nk, r.Tk, r.Data)
		if err != nil {
			log.Println(err)
			return
		}
	}
	
	return 
}

func (d DB) GetLocationsClusters ()(locInfos LocationsInfoSets,  err error) {
	log.Println("GetLocationsClusters")
	var query = "SELECT location, serial FROM measurements GROUP BY location, serial;"
	rows, err := d.Query(query)
	
	if err != nil {
                log.Println(err)
                return
        }
        defer rows.Close()

	x := make(map[string]Location)	
	
        for rows.Next() {        
		var l string 
		var serial string
                
		err = rows.Scan(&l, &serial)
                //log.Println(l,clusterid)
	

		if err != nil {
			log.Println(err)
			return
		} else {
			locinfo, ok := x[l]
			if ok {
				ss, _ := d.GetSerialInfo(l,serial)
				locinfo.Serials = append(locinfo.Serials,ss)
				x[l] = locinfo
				//log.Println(x)
			} else {
				locinfo.Name = l
				ss, _ := d.GetSerialInfo(l,serial)
				locinfo.Serials = append(locinfo.Serials,ss)
				x[l] = locinfo
				//log.Println(x)
			}
		}
        }

	for _, value := range x {
		locInfos = append(locInfos, value)
	}	

        return locInfos,rows.Err()
}

func (d DB) GetSerialInfo(loc string, ser string) (serial Serial, err error){
	log.Println("GetSerialInfo ", loc, ser)
	var query = "SELECT register, type FROM measurements WHERE location=? AND serial=? GROUP BY register, type;"
	rows, err := d.Query(query,loc,ser)
	
	if err != nil {
		log.Println(err)
		return
	}	
	defer rows.Close()

	for rows.Next() {
		r := Register{}
		err = rows.Scan(&r.Name, &r.Type)
		if err != nil {
			log.Println(err)
		} else {
			serial.Registers = append(serial.Registers, r)
		}
	}
	
	serial.Name = ser

	return
}

func (d DB) SetNewSerial(u_id int64, serial string) (err error) {
	tx, err := d.Begin()
	if err != nil {
		return
	}

	var query = "INSERT INTO serials (serial,user_id) VALUES(?,?);"
	stmt, err := tx.Prepare(query)
	if err != nil {
		return
	}
	
	defer func () {
		if err == nil {
			tx.Commit()
			log.Println("Commit Serial")
		} else {
			tx.Rollback()
			log.Println("Rollback Serial")
		}
		stmt.Close()
	}()

	_, err = stmt.Exec(serial,u_id)
	return
}

func (d DB) GetSerials() (ss []Serial, err error) {
	var query = "SELECT serial, user_id, id FROM serials;"
	rows, err := d.Query(query)
	
	if err != nil {
		log.Println(err)
		return
	}	
	defer rows.Close()
	
	for rows.Next() {
		s := Serial{}
		err = rows.Scan(&s.Name, &s.User_Id, &s.Id)
		if err != nil {
			log.Println(err)
		} else {
			ss = append(ss, s)
		}
	}
	return	
}

func (d DB) GetUserWithId(id int64) (u User,err error) {
	var query = "SELECT pwh, pws, uname, id FROM users WHERE id=?;"
	err = d.QueryRow(query,id).Scan(&u.Hash,&u.Salt,&u.UserName,&u.ID)
        if err != nil {
                log.Println(err)
                return
        }
	return	
}

func (d DB) SetNewUser(un string, pw string)(id int64, err error) {
	salt, err := Crypt([]byte(pw))
	if err != nil {
		return
	}

	hash := HashPassword([]byte(pw), salt)
	tx, err := d.Begin()
	if err != nil {
		return
	}	
	
	var query = "INSERT INTO users (pwh,pws,uname) VALUES (?, ?, ?);"
	stmt, err := tx.Prepare(query)
	if err != nil {
		return
	}
	
	defer func () {
		if err == nil {
			tx.Commit()
			log.Println("Commit User")
		} else {
			tx.Rollback()
			log.Println("Rollback User")
		}
		stmt.Close()
	}()

	r, err := stmt.Exec(hash,salt,un)
	if err != nil {
		return
	}	

	id, err = r.LastInsertId()

	return
}


