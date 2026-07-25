package community

// anonName is the anonymized author label shown for learner-authored posts and
// replies. Admin-authored content is labelled "Admin" by the handlers instead.
func anonName(_ string) string {
	return "Learner"
}
