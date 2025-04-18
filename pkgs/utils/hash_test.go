package utils

import (
	"testing"
)

func TestSBOM_Matches(t *testing.T) {
	type args struct {
		o SBOM
	}
	tests := []struct {
		name    string
		init    func(t *testing.T) SBOM
		inspect func(r SBOM, t *testing.T) //inspects receiver after test run

		args func(t *testing.T) args

		wantErr    bool
		inspectErr func(err error, t *testing.T) //use for more precise error evaluation after test
	}{
		{
			"matching",
			func(t *testing.T) SBOM {
				return SBOM{
					Tools: Hashes{
						{
							Name: "a1",
							Hash: "ah1",
							Params: Hashes{
								{
									Name: "p1",
									Hash: "ph1",
								},
							},
						},
					},
				}
			},
			nil,
			func(*testing.T) args {
				return args{
					SBOM{
						Tools: Hashes{
							{
								Name: "a1",
								Hash: "ah1",
								Params: Hashes{
									{
										Name: "p1",
										Hash: "ph1",
									},
								},
							},
						},
					},
				}
			},
			false,
			nil,
		},
		{
			"no params",
			func(t *testing.T) SBOM {
				return SBOM{
					Tools: Hashes{
						{
							Name: "a1",
							Hash: "ah1",
						},
					},
				}
			},
			nil,
			func(*testing.T) args {
				return args{
					SBOM{
						Tools: Hashes{
							{
								Name: "a1",
								Hash: "ah1",
							},
						},
					},
				}
			},
			false,
			nil,
		},
		{
			"empty",
			func(t *testing.T) SBOM {
				return SBOM{}
			},
			nil,
			func(*testing.T) args {
				return args{
					SBOM{},
				}
			},
			false,
			nil,
		},
		{
			"missing param",
			func(t *testing.T) SBOM {
				return SBOM{
					Tools: Hashes{
						{
							Name: "a1",
							Hash: "ah1",
							Params: Hashes{
								{
									Name: "p1",
									Hash: "ph1",
								},
								{
									Name: "p2",
									Hash: "ph2",
								},
							},
						},
					},
				}
			},
			nil,
			func(*testing.T) args {
				return args{
					SBOM{
						Tools: Hashes{
							{
								Name: "a1",
								Hash: "ah1",
								Params: Hashes{
									{
										Name: "p1",
										Hash: "ph1",
									},
								},
							},
						},
					},
				}
			},
			true,
			func(err error, t *testing.T) {
				want := "invalid tool: 'a1': invalid param: invalid len. left: 2 right: 1"
				if err.Error() != want {
					t.Logf("invalid err. want: %s got: %s", want, err.Error())
					t.Fail()
				}
			},
		},
		{
			"extra param",
			func(t *testing.T) SBOM {
				return SBOM{
					Tools: Hashes{
						{
							Name: "a1",
							Hash: "ah1",
							Params: Hashes{
								{
									Name: "p1",
									Hash: "ph1",
								},
							},
						},
					},
				}
			},
			nil,
			func(*testing.T) args {
				return args{
					SBOM{
						Tools: Hashes{
							{
								Name: "a1",
								Hash: "ah1",
								Params: Hashes{
									{
										Name: "p1",
										Hash: "ph1",
									},
									{
										Name: "p2",
										Hash: "ph2",
									},
								},
							},
						},
					},
				}
			},
			true,
			func(err error, t *testing.T) {
				want := "invalid tool: 'a1': invalid param: invalid len. left: 1 right: 2"
				if err.Error() != want {
					t.Logf("invalid err. want: %s got: %s", want, err.Error())
					t.Fail()
				}
			},
		},
		{
			"missing tool",
			func(t *testing.T) SBOM {
				return SBOM{
					Tools: Hashes{
						{
							Name: "a1",
							Hash: "ah1",
							Params: Hashes{
								{
									Name: "p1",
									Hash: "ph1",
								},
							},
						},
						{
							Name: "a2",
							Hash: "ah2",
							Params: Hashes{
								{
									Name: "p1",
									Hash: "ph1",
								},
							},
						},
					},
				}
			},
			nil,
			func(*testing.T) args {
				return args{
					SBOM{
						Tools: Hashes{
							{
								Name: "a1",
								Hash: "ah1",
								Params: Hashes{
									{
										Name: "p1",
										Hash: "ph1",
									},
								},
							},
						},
					},
				}
			},
			true,
			func(err error, t *testing.T) {
				want := "invalid tool: invalid len. left: 2 right: 1"
				if err.Error() != want {
					t.Logf("invalid err. want: %s got: %s", want, err.Error())
					t.Fail()
				}
			},
		},
		{
			"extra tool",
			func(t *testing.T) SBOM {
				return SBOM{
					Tools: Hashes{
						{
							Name: "a1",
							Hash: "ah1",
							Params: Hashes{
								{
									Name: "p1",
									Hash: "ph1",
								},
							},
						},
					},
				}
			},
			nil,
			func(*testing.T) args {
				return args{
					SBOM{
						Tools: Hashes{
							{
								Name: "a1",
								Hash: "ah1",
								Params: Hashes{
									{
										Name: "p1",
										Hash: "ph1",
									},
								},
							},
							{
								Name: "a2",
								Hash: "ah2",
								Params: Hashes{
									{
										Name: "p1",
										Hash: "ph1",
									},
								},
							},
						},
					},
				}
			},
			true,
			func(err error, t *testing.T) {
				want := "invalid tool: invalid len. left: 1 right: 2"
				if err.Error() != want {
					t.Logf("invalid err. want: %s got: %s", want, err.Error())
					t.Fail()
				}
			},
		},
		{
			"invalid tool hash",
			func(t *testing.T) SBOM {
				return SBOM{
					Tools: Hashes{
						{
							Name: "a1",
							Hash: "ah1",
							Params: Hashes{
								{
									Name: "p1",
									Hash: "ph1",
								},
							},
						},
					},
				}
			},
			nil,
			func(*testing.T) args {
				return args{
					SBOM{
						Tools: Hashes{
							{
								Name: "a1",
								Hash: "NOT_ah1",
								Params: Hashes{
									{
										Name: "p1",
										Hash: "ph1",
									},
								},
							},
						},
					},
				}
			},
			true,
			func(err error, t *testing.T) {
				want := "invalid tool: 'a1': hash mismatch"
				if err.Error() != want {
					t.Logf("invalid err. want: %s got: %s", want, err.Error())
					t.Fail()
				}
			},
		},
		{
			"invalid param hash",
			func(t *testing.T) SBOM {
				return SBOM{
					Tools: Hashes{
						{
							Name: "a1",
							Hash: "ah1",
							Params: Hashes{
								{
									Name: "p1",
									Hash: "ph1",
								},
							},
						},
					},
				}
			},
			nil,
			func(*testing.T) args {
				return args{
					SBOM{
						Tools: Hashes{
							{
								Name: "a1",
								Hash: "ah1",
								Params: Hashes{
									{
										Name: "p1",
										Hash: "NOT-ph1",
									},
								},
							},
						},
					},
				}
			},
			true,
			func(err error, t *testing.T) {
				want := "invalid tool: 'a1': invalid param: 'p1': hash mismatch"
				if err.Error() != want {
					t.Logf("invalid err. want: %s got: %s", want, err.Error())
					t.Fail()
				}
			},
		},
		{
			"tool name missing",
			func(t *testing.T) SBOM {
				return SBOM{
					Tools: Hashes{
						{
							Name: "a1",
							Hash: "ah1",
							Params: Hashes{
								{
									Name: "p1",
									Hash: "ph1",
								},
							},
						},
					},
				}
			},
			nil,
			func(*testing.T) args {
				return args{
					SBOM{
						Tools: Hashes{
							{
								Name: "b1",
								Hash: "bh1",
								Params: Hashes{
									{
										Name: "p1",
										Hash: "ph1",
									},
								},
							},
						},
					},
				}
			},
			true,
			func(err error, t *testing.T) {
				want := "invalid tool: 'a1': missing"
				if err.Error() != want {
					t.Logf("invalid err. want: %s got: %s", want, err.Error())
					t.Fail()
				}
			},
		},
		{
			"param name missing left",
			func(t *testing.T) SBOM {
				return SBOM{
					Tools: Hashes{
						{
							Name: "a1",
							Hash: "ah1",
							Params: Hashes{
								{
									Name: "p1",
									Hash: "ph1",
								},
							},
						},
					},
				}
			},
			nil,
			func(*testing.T) args {
				return args{
					SBOM{
						Tools: Hashes{
							{
								Name: "a1",
								Hash: "ah1",
								Params: Hashes{
									{
										Name: "q1",
										Hash: "qh1",
									},
								},
							},
						},
					},
				}
			},
			true,
			func(err error, t *testing.T) {
				want := "invalid tool: 'a1': invalid param: 'p1': missing"
				if err.Error() != want {
					t.Logf("invalid err. want: %s got: %s", want, err.Error())
					t.Fail()
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tArgs := tt.args(t)

			receiver := tt.init(t)
			err := receiver.Matches(tArgs.o)

			if tt.inspect != nil {
				tt.inspect(receiver, t)
			}

			if (err != nil) != tt.wantErr {
				t.Fatalf("SBOM.Matches error = %v, wantErr: %t", err, tt.wantErr)
			}

			if tt.inspectErr != nil {
				tt.inspectErr(err, t)
			}
		})
	}
}
