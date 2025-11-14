package database

import (
	"fmt"
	"log"
	"strings"
)

func (s *PostgreSQLStore) Initialize() error {
	// Init all enums
	log.Println("Initializing PostgresSQL Database.", "Initializing Enums")
	if err := s.InitEnums(); err != nil {
		return err
	}
	// Init all tables
	log.Println("Initializing PostgresSQL Database.", "Initializing Tables")
	if err := s.InitTables(); err != nil {
		return err
	}
	// Print relationships
	log.Println("Initializing PostgresSQL Database.", "Printing Relationships")
	s.PrintAllRelationships()
	return nil
}

func (s *PostgreSQLStore) InitEnums() error {
	// Init all the enums
	query := `
		DO $$
		BEGIN
           	IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'user_roles') THEN
				CREATE TYPE user_roles AS ENUM ('admin', 'doctor', 'assistant');
           	END IF;
		END $$;

		DO $$
		BEGIN
           	IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'gender') THEN
				CREATE TYPE gender AS ENUM ('male', 'female', 'non-binary', 'other');
           	END IF;
		END $$;

		DO $$
		BEGIN
           	IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'report_type') THEN
				CREATE TYPE report_type AS ENUM ('normal', 'abnormal');
           	END IF;
		END $$;

		DO $$
		BEGIN
           	IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'notification_type') THEN
				CREATE TYPE notification_type AS ENUM('referral', 'comment');
           	END IF;
		END $$;

		DO $$
		BEGIN
           	IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'todo_status') THEN
				CREATE TYPE todo_status AS ENUM('pending', 'completed', 'unable to do');
           	END IF;
		END $$;
	`
	_, err := s.db.Exec(query)

	return err
}

func (s *PostgreSQLStore) InitTables() error {
	//
	// Init all the tables
	//

	// users table
	users_table := `
	CREATE TABLE IF NOT EXISTS users (
        id INTEGER PRIMARY KEY GENERATED ALWAYS AS IDENTITY,
        role user_roles,
        name VARCHAR(512) NOT NULL,
        speciality VARCHAR(255),
        email VARCHAR(512) UNIQUE NOT NULL,
        password TEXT NOT NULL,
        image VARCHAR(255),
        gender gender,
        country_code INT NOT NULL,
        mobile_no BIGINT NOT NULL,
        whatsapp_no BIGINT NOT NULL,
        country VARCHAR(120) NOT NULL,
        state VARCHAR(255) NOT NULL,
        city VARCHAR(255) NOT NULL,
        hospital_name VARCHAR(512) NOT NULL,
        created_at TIMESTAMP
	);
	`
	// doctor_assistant table
	doctor_assistant_table := `
	CREATE TABLE IF NOT EXISTS doctor_assistant (
		doctor_id INTEGER UNIQUE REFERENCES users(id) ON DELETE CASCADE,
		assistant_id INTEGER UNIQUE REFERENCES users(id) ON DELETE CASCADE,
		CONSTRAINT doctor_assistant_pk PRIMARY KEY (doctor_id, assistant_id)
    );
	`

	// patient table
	patient_table := `
	CREATE TABLE IF NOT EXISTS patient (
        id INTEGER PRIMARY KEY GENERATED ALWAYS AS IDENTITY,
        doctor_id INTEGER REFERENCES users(id) ON DELETE CASCADE,
        image VARCHAR(512) DEFAULT 'patient.jpg',
        name VARCHAR(512) NOT NULL,
        gender gender,
        weight INTEGER,
        height INTEGER,
        age INTEGER,
        dob DATE,
        address TEXT,
        email VARCHAR(512) UNIQUE NOT NULL,
        country_code INTEGER NOT NULL,
        mobile_no BIGINT NOT NULL,
        whatsapp_no BIGINT NOT NULL,
        emergency_contact JSONB,
        medical_conditions VARCHAR(255)[],
        allergies VARCHAR(255)[],
        previous_surgeries VARCHAR(255)[]
    );
	`

	// report case table
	report_case_table := `
    CREATE TABLE IF NOT EXISTS report_case (
    	patient_id INTEGER REFERENCES patient(id) ON DELETE CASCADE,
    	name VARCHAR(255) NOT NULL,
    	id INTEGER PRIMARY KEY GENERATED ALWAYS AS IDENTITY,
    	UNIQUE(patient_id, name)
    );
	`

	// medical report table
	medical_report_table := `
	CREATE TABLE IF NOT EXISTS medical_report  (
		case_id INTEGER REFERENCES report_case(id) ON DELETE CASCADE,
		name VARCHAR(255) NOT NULL,
		source VARCHAR(512) NOT NULL,
		date DATE NOT NULL,
		url VARCHAR(512) NOT NULL,
		report_type report_type,
		CONSTRAINT medical_report_pk PRIMARY KEY (name, case_id),
		UNIQUE(case_id, name)
	);
	`

	// comments table
	comments_table := `
	CREATE TABLE IF NOT EXISTS comments (
		id INTEGER PRIMARY KEY GENERATED ALWAYS AS IDENTITY,
		patient_id INTEGER REFERENCES patient(id) ON DELETE CASCADE,
		sender INTEGER REFERENCES users(id) ON DELETE CASCADE,
		message VARCHAR(512) NOT NULL
	);
	`

	// notifications table
	notifications_table := `
	CREATE TABLE IF NOT EXISTS notifications (
		id INTEGER PRIMARY KEY GENERATED ALWAYS AS IDENTITY,
		sender INTEGER REFERENCES users(id) ON DELETE CASCADE,
		sender_name VARCHAR(255) NOT NULL,
		receiver INTEGER REFERENCES users(id) ON DELETE CASCADE,
		receiver_name VARCHAR(255) NOT NULL,
		type notification_type ,
		url VARCHAR(100)
	);
	`
	// TODO REMOVE THIS
	// DEVELOPMENT TESTING
	todo_table := `
	CREATE TABLE IF NOT EXISTS todo (
		id BIGINT PRIMARY KEY GENERATED ALWAYS AS IDENTITY,
		name VARCHAR(100) NOT NULL UNIQUE,
		description TEXT NOT NULL,
		due DATE NOT NULL,
		status todo_status DEFAULT 'pending'
	);
	`
	//

	all_tables := strings.Join([]string{users_table, doctor_assistant_table, patient_table, report_case_table, medical_report_table, comments_table, notifications_table, todo_table}, "")

	_, err := s.db.Exec(all_tables)
	return err
}

func (s *PostgreSQLStore) PrintAllRelationships() {
	relationships := map[string]string{
		"doctor_assistant": "doctor_id	-> users(id), assistant_id -> users(id)",
		"patient":          "doctor_id	-> users(id)",
		"report_case":      "patient_id -> patient(id)",
		"medical_report":   "case_id	-> report_case(id)",
		"comments":         "patient_id -> patient(id), sender -> users(id)",
		"notifications":    "sender		-> users(id), receiver -> users(id)",
	}

	// Print the relationships
	for table, relationship := range relationships {
		fmt.Printf("Relationships for %s table:\n", table)
		fmt.Println(relationship)
		fmt.Println()
	}

}
