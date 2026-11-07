package cases

// Grid returns every case of the profile with its projects and verdicts, by
// key: the rows of the cases-by-projects grid. Each row is the case's own
// Get, so the grid cannot disagree with the case page.
//
// ponytail: one query per case. Fine for the hundreds an analyst keeps; turn
// it into one join if a profile ever holds thousands.
func (s *Store) Grid(profileID string) ([]Detail, error) {
	list, err := s.List(profileID)
	if err != nil {
		return nil, err
	}
	out := make([]Detail, 0, len(list))
	for _, c := range list {
		d, err := s.Get(profileID, c.ID)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, nil
}
