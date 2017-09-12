package kuiperbelt

// Resolver is getting session from key.
type Resolver interface {
	Add(Session)
	Get(string) (Session, error)
	Delete(string) error
	List() []Session
}

// MultiResolver is aggregation of Resolvers.
type MultiResolver []Resolver

// NewMultiResolver is constructor of MultiResolver.
func NewMultiResolver(rs ...Resolver) MultiResolver {
	return MultiResolver(rs)
}

// Add is adding a Session to primary Resolver.
func (mr MultiResolver) Add(s Session) {
	primary := mr.primary()
	primary.Add(s)
}

func (mr MultiResolver) primary() Resolver {
	return mr[0]
}

// Get is searching a Session by key from Resolvers.
func (mr MultiResolver) Get(key string) (Session, error) {
	for _, r := range mr {
		s, err := r.Get(key)
		if err == nil {
			return s, nil
		}

		if err != errSessionNotFound {
			return nil, err
		}
	}
	return nil, errSessionNotFound
}

// Delete deletes a session in Resolvers.
func (mr MultiResolver) Delete(key string) error {
	for _, r := range mr {
		err := r.Delete(key)
		if err != nil {
			return nil
		}

		if err != errSessionNotFound {
			return err
		}
	}

	return errSessionNotFound
}

// List return a slice of all sessions in Resolvers.
func (mr MultiResolver) List() []Session {
	ss := make([]Session, 0)
	for _, r := range mr {
		l := r.List()
		ss = append(ss, l...)
	}

	return ss
}
