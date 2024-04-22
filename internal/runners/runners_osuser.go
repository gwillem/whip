package runners

import "os/user"

// Stub os/user so we can override in tests

type OsUser interface {
	Current() (*user.User, error)
	Lookup(username string) (*user.User, error)
	// LookupId(uid string) (*user.User, error)
	// LookupGroupId(gid string) (*user.Group, error)
	LookupGroup(groupname string) (*user.Group, error)
}

var osUser OsUser = realOsUser{}

type realOsUser struct{}

func (realOsUser) Current() (*user.User, error) {
	return user.Current()
}

func (realOsUser) Lookup(username string) (*user.User, error) {
	return user.Lookup(username)
}

func (realOsUser) LookupGroup(groupname string) (*user.Group, error) {
	return user.LookupGroup(groupname)
}

type stubOsUser struct {
	current  *user.User
	group    *user.Group
	userMap  map[string]*user.User
	groupMap map[string]*user.Group
}

func (u stubOsUser) Current() (*user.User, error) {
	return u.current, nil
}

func (u stubOsUser) Lookup(username string) (*user.User, error) {
	if u.userMap != nil {
		if user, ok := u.userMap[username]; ok {
			return user, nil
		}
	}
	return nil, user.UnknownUserError(username)
}

func (u stubOsUser) LookupGroup(groupname string) (*user.Group, error) {
	if u.groupMap != nil {
		if group, ok := u.groupMap[groupname]; ok {
			return group, nil
		}
	}
	return nil, user.UnknownGroupError(groupname)
}
