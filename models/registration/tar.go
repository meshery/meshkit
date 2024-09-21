package registration

type Tar struct {
	_ string
}

func (t Tar) PkgUnit(regErrStore RegistrationErrorStore) (PackagingUnit, error) {
	pkg := PackagingUnit{}
	return pkg, nil
}
