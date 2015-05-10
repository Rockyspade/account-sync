package accountsync

import "fmt"

type Owner struct {
	Type         string
	User         *User
	Organization *Organization
}

func (o *Owner) Key() string {
	switch o.Type {
	case "user":
		return fmt.Sprintf("user-%v", o.User.ID)
	case "organization":
		return fmt.Sprintf("organization-%v", o.Organization.ID)
	}
	panic(fmt.Errorf("invalid owner type %q", o.Type))
}

func (o *Owner) String() string {
	switch o.Type {
	case "user":
		return o.User.Login.String
	case "organization":
		return o.Organization.Login.String
	}
	panic(fmt.Errorf("invalid owner type %q", o.Type))
}
