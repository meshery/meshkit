package registration

type Tar struct {
	_ string
}

func (t Tar) PkgUnit(regErrStore RegistrationErrorStore) (packagingUnit, error) {
	pkg := packagingUnit{}
	return pkg, nil
}
