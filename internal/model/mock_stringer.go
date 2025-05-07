package model

type MockStringer string

func (m MockStringer) String() string {
	return string(m)
}
