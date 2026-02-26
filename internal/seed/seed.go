// Package seed initializes the database with default data at startup.
// It creates a "default" group, an "admin" user (password: admin) if no users exist,
// and assigns every app that has no groups to the default group.
// Run is called once from main after opening the store; there is no HTTP API for groups/users.
package seed

import (
	"log"

	"piaflow/internal/auth"
	"piaflow/internal/config"
	"piaflow/internal/store"
)

// Run ensures a default group and admin user exist, and assigns apps without groups to the default group.
// Idempotent: safe to call on every startup; only creates missing data.
func Run(st *store.Store, apps []config.App) {
	groups, err := st.ListGroups()
	if err != nil {
		log.Printf("seed: list groups: %v", err)
		return
	}
	var defaultGroupID int64
	if len(groups) == 0 {
		defaultGroupID, err = st.CreateGroup("default")
		if err != nil {
			log.Printf("seed: create default group: %v", err)
			return
		}
		log.Printf("seed: created group 'default' (id=%d)", defaultGroupID)
	} else {
		for _, g := range groups {
			if g.Name == "default" {
				defaultGroupID = g.ID
				break
			}
		}
		if defaultGroupID == 0 {
			defaultGroupID = groups[0].ID
		}
	}

	users, err := st.ListUsers()
	if err != nil {
		log.Printf("seed: list users: %v", err)
		return
	}
	if len(users) == 0 {
		hash, err := auth.HashPassword("admin")
		if err != nil {
			log.Printf("seed: hash password: %v", err)
			return
		}
		adminID, err := st.CreateUser("admin", hash)
		if err != nil {
			log.Printf("seed: create admin user: %v", err)
			return
		}
		_ = st.SetUserGroups(adminID, []int64{defaultGroupID})
		log.Printf("seed: created user 'admin' (password: admin)")
	}

	for _, app := range apps {
		ids, _ := st.AppGroupIDs(app.ID)
		if len(ids) == 0 {
			_ = st.SetAppGroups(app.ID, []int64{defaultGroupID})
		}
	}
}
