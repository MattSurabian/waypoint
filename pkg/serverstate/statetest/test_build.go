package statetest

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/hashicorp/waypoint/pkg/server/gen"
	serverptypes "github.com/hashicorp/waypoint/pkg/server/ptypes"
	"github.com/hashicorp/waypoint/pkg/serverstate"
)

func init() {
	tests["build"] = []testFunc{
		TestBuild,
	}
}

func TestBuild(t *testing.T, factory Factory, restartF RestartFactory) {
	t.Run("CRUD operations", func(t *testing.T) {
		require := require.New(t)

		s := factory(t)
		defer s.Close()

		// Write project
		ref := &pb.Ref_Project{Project: "foo"}
		require.NoError(s.ProjectPut(serverptypes.TestProject(t, &pb.Project{
			Name: ref.Project,
		})))

		// Has no apps
		{
			resp, err := s.ProjectGet(ref)
			require.NoError(err)
			require.NotNil(resp)
			require.Empty(resp.Applications)
		}

		app := &pb.Ref_Application{
			Project:     ref.Project,
			Application: "testapp",
		}

		ws := &pb.Ref_Workspace{
			Workspace: "default",
		}

		// Add
		err := s.BuildPut(false, serverptypes.TestBuild(t, &pb.Build{
			Id:          "d1",
			Application: app,
			Workspace:   ws,
			Status: &pb.Status{
				State:     pb.Status_SUCCESS,
				StartTime: timestamppb.Now(),
			},
		}))
		require.NoError(err)

		// Can read
		{
			resp, err := s.BuildGet(&pb.Ref_Operation{
				Target: &pb.Ref_Operation_Id{
					Id: "d1",
				},
			})
			require.NoError(err)
			require.NotNil(resp)
		}

		// Can read latest
		{
			resp, err := s.BuildLatest(app, &pb.Ref_Workspace{Workspace: "default"})
			require.NoError(err)
			require.NotNil(resp)
		}

		// Update
		ts := timestamppb.Now()
		err = s.BuildPut(true, serverptypes.TestBuild(t, &pb.Build{
			Id:          "d1",
			Application: app,
			Workspace:   ws,
			Status: &pb.Status{
				State:        pb.Status_SUCCESS,
				StartTime:    ts,
				CompleteTime: ts,
			},
		}))
		require.NoError(err)

		{
			resp, err := s.BuildGet(&pb.Ref_Operation{
				Target: &pb.Ref_Operation_Id{
					Id: "d1",
				},
			})
			require.NoError(err)
			require.NotNil(resp)

			require.Equal(ts.AsTime(), resp.Status.CompleteTime.AsTime())
		}

		// Add another and see Latset change
		// Add
		err = s.BuildPut(false, serverptypes.TestBuild(t, &pb.Build{
			Id:          "d2",
			Application: app,
			Workspace:   ws,
			Status: &pb.Status{
				State:        pb.Status_SUCCESS,
				StartTime:    timestamppb.Now(),
				CompleteTime: timestamppb.New(ts.AsTime().Add(2 * time.Second)),
			},
		}))
		require.NoError(err)

		{
			resp, err := s.BuildLatest(app, &pb.Ref_Workspace{Workspace: "default"})
			require.NoError(err)
			require.NotNil(resp)
			require.Equal("d2", resp.Id)
		}

		{
			resp, err := s.BuildList(app)
			require.NoError(err)

			require.Len(resp, 2)
		}

		/*
					TODO: singleprocess/state's usage of Desc is broken.
			{
				resp, err := s.BuildList(app, serverstate.ListWithOrder(&pb.OperationOrder{
					Order: pb.OperationOrder_START_TIME,
					Desc:  false,
					Limit: 1,
				}))
				require.NoError(err)

				require.Len(resp, 1)

				require.Equal("d1", resp[0].Id)
			}
		*/

		{
			resp, err := s.BuildList(app, serverstate.ListWithOrder(&pb.OperationOrder{
				Order: pb.OperationOrder_START_TIME,
				Desc:  true,
				Limit: 1,
			}))
			require.NoError(err)

			require.Len(resp, 1)

			require.Equal("d2", resp[0].Id)
		}

		err = s.BuildPut(false, serverptypes.TestBuild(t, &pb.Build{
			Id:          "d3",
			Application: app,
			Workspace:   ws,
			Status: &pb.Status{
				State:     pb.Status_ERROR,
				StartTime: timestamppb.Now(),
			},
		}))
		require.NoError(err)

		{
			resp, err := s.BuildList(app)
			require.NoError(err)

			require.Len(resp, 3)
		}

		{
			resp, err := s.BuildList(app,
				serverstate.ListWithOrder(&pb.OperationOrder{
					Order: pb.OperationOrder_START_TIME,
					Desc:  true,
				}),
				serverstate.ListWithStatusFilter(&pb.StatusFilter{
					Filters: []*pb.StatusFilter_Filter{
						{
							Filter: &pb.StatusFilter_Filter_State{
								State: pb.Status_ERROR,
							},
						},
					},
				}),
			)
			require.NoError(err)

			require.Len(resp, 1)

			require.Equal("d3", resp[0].Id)
		}
		{
			resp, err := s.BuildList(app,
				serverstate.ListWithOrder(&pb.OperationOrder{
					Order: pb.OperationOrder_START_TIME,
					Desc:  true,
				}),
				serverstate.ListWithStatusFilter(
					&pb.StatusFilter{
						Filters: []*pb.StatusFilter_Filter{
							{
								Filter: &pb.StatusFilter_Filter_State{
									State: pb.Status_ERROR,
								},
							},
						},
					},
					&pb.StatusFilter{
						Filters: []*pb.StatusFilter_Filter{
							{
								Filter: &pb.StatusFilter_Filter_State{
									State: pb.Status_SUCCESS,
								},
							},
						},
					},
				),
			)
			require.NoError(err)

			require.Len(resp, 3)
		}
	})

}
