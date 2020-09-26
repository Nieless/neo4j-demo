package main

import (
	"fmt"
	"github.com/neo4j/neo4j-go-driver/neo4j"
	uuid "github.com/satori/go.uuid"
	"time"
)

func main() {
	// configForNeo4j35 := func(conf *neo4j.Config) {}
	configForNeo4j40 := func(conf *neo4j.Config) { conf.Encrypted = false }

	driver, err := neo4j.NewDriver("bolt://localhost:7687", neo4j.BasicAuth("neo4j", "neo4j", ""), configForNeo4j40)
	if err != nil {
		panic(err)
	}

	// handle driver lifetime based on your application lifetime requirements
	// driver's lifetime is usually bound by the application lifetime, which usually implies one driver instance per application
	defer driver.Close()

	// For multi-database support, set sessionConfig.DatabaseName to requested database
	sessionConfig := neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite}
	session, err := driver.NewSession(sessionConfig)
	if err != nil {
		panic(err)
	}

	defer session.Close()

	user := &User{
		ID:   uuid.NewV4().String(),
		Name: "testUser",
	}

	if err := user.add(session); err != nil {
		panic(err)
	}

	et := &EntitlementDefinition{
		ID:            uuid.NewV4().String(),
		UserID:        user.ID,
		EffectiveDate: time.Now().Unix(),
		TermDate:      time.Now().Unix(),
		CreatedBy:     "test",
		CreatedTS:     time.Now().Unix(),
	}

	etService := &entitlementDefinitionRepositoryImpl{
		session: session,
	}

	if err := etService.add(et); err != nil {
		panic(err)
	}

	ets, err := etService.getAll()
	if err != nil {
		panic(err)
	}

	fmt.Printf("found ets: %+v\n", ets)

	etd, err := etService.get(et.ID)
	if err != nil {
		panic(err)
	}

	fmt.Printf("found et: %+v\n", etd)
}

type EntitlementDefinition struct {
	ID            string   `json:"entitlementID"`
	UserID        string   `json:"userID"`
	EffectiveDate int64    `json:"effectiveDate"`
	TermDate      int64    `json:"termDate"`
	CreatedBy     string   `json:"createdBy"`
	CreatedTS     int64    `json:"createdTs"`
	ModifiedBy    *string  `json:"modifiedBy"`
	ModifiedTs    *int64   `json:"modifiedTs"`
	Roles         []string `json:"roles"`
	//AssignedRoles []RoleDefinition `json:"assignedRoles,omitempty"`
}

type entitlementDefinitionRepository interface {
	add(et *EntitlementDefinition) error
}

type entitlementDefinitionRepositoryImpl struct {
	session neo4j.Session
}

func (impl *entitlementDefinitionRepositoryImpl) add(et *EntitlementDefinition) error {

	_, err := impl.session.WriteTransaction(func(transaction neo4j.Transaction) (interface{}, error) {
		cypherQuery := fmt.Sprintf(`
				CREATE
					(et:entitlement)
				SET
					et.entitlementID = $id,
					et.effectiveDate = $effectiveDate,
					et.termDate = $termDate,
					et.createdBy =  $createdBy,
					et.createdTs =  $createdTs,
					et.modifiedBy = $modifiedBy,
					et.modifiedTs = $modifiedTs`)

		params := map[string]interface{}{
			"id":            et.ID,
			"effectiveDate": et.EffectiveDate,
			"termDate":      et.TermDate,
			"createdBy":     et.CreatedBy,
			"createdTs":     et.CreatedTS,
			"modifiedBy":    et.CreatedBy,
			"modifiedTs":    et.ModifiedTs,
		}

		result, err := transaction.Run(cypherQuery, params)
		if err != nil {
			return nil, err
		}

		cypherQuery = fmt.Sprintf(`
			MATCH 
				(et:entitlement), (usr:user) 
			WHERE
				et.entitlementID = $etID AND usr.userID = $userID
			CREATE (et)-[r:ASSOCIATED_TO]->(usr)
			RETURN et`)
		params = map[string]interface{}{"etID": et.ID, "userID": et.UserID}

		result, err = transaction.Run(cypherQuery, params)
		if err != nil {
			_ = transaction.Rollback()
			return nil, err
		}

		if result.Next() {
			return result.Record().GetByIndex(0), nil
		}

		if result.Err() != nil {
			_ = transaction.Rollback()
			return nil, result.Err()
		}

		err = transaction.Commit()
		return nil, err
	})

	return err
}

func (impl *entitlementDefinitionRepositoryImpl) getAll() ([]EntitlementDefinition, error) {

	cypherQuery := fmt.Sprintf(`
			MATCH 
				(et:entitlement)
			RETURN 
				et.entitlementID`)

	params := map[string]interface{}{}
	result, err := impl.session.Run(cypherQuery, params)
	if err != nil {
		return nil, err
	}

	ets := make([]EntitlementDefinition, 0)
	for result.Next() {
		var et EntitlementDefinition
		et.ID = result.Record().GetByIndex(0).(string)
		ets = append(ets, et)
	}

	return ets, result.Err()
}

func (impl *entitlementDefinitionRepositoryImpl) get(etId string) (*EntitlementDefinition, error) {
	et := &EntitlementDefinition{}

	cypherQuery := fmt.Sprintf(`
		MATCH 
			(et:entitlement)
		WHERE
			et.entitlementID = $entitlementID
		RETURN et.entitlementID`)
	params := map[string]interface{}{"entitlementID": etId}

	result, err := impl.session.Run(cypherQuery, params)
	if err != nil {
		return nil, err
	}

	if result.Next() {
		et.ID = result.Record().GetByIndex(0).(string)
		return et, nil
	}

	return nil, result.Err()
}

type User struct {
	ID   string `json:"userID"`
	Name string `json:"name"`
}

func (user *User) add(session neo4j.Session) error {
	cypherQuery := fmt.Sprintf(`
				CREATE
					(user:user)
				SET
					user.userID = $id,
					user.name = $name
				RETURN
					user`)

	params := map[string]interface{}{
		"id":   user.ID,
		"name": user.Name,
	}
	result, err := session.Run(cypherQuery, params)
	if err != nil {
		return err
	}

	if result.Next() {
		//return result.Record().GetByIndex(0), nil
	}

	return err
}
