module SideBarTests exposing (all)

import Expect
import Message.Effects as Effects
import Message.Message exposing (DomID(..), Message(..))
import RemoteData
import ScreenSize
import Set
import SideBar.SideBar as SideBar
import Test exposing (Test, describe, test)


all : Test
all =
    describe "SideBar"
        [ describe ".update"
            [ test "asks browser for viewport when hovering a pipeline link" <|
                \_ ->
                    let
                        domID =
                            SideBarPipeline
                                { teamName = "team"
                                , pipelineName = "pipeline"
                                }
                    in
                    { expandedTeams = Set.fromList [ "team" ]
                    , pipelines =
                        RemoteData.Success
                            [ { id = 0
                              , name = "pipeline"
                              , paused = False
                              , public = True
                              , teamName = "team"
                              , groups = []
                              }
                            ]
                    , hovered = Nothing
                    , isSideBarOpen = True
                    , screenSize = ScreenSize.Desktop
                    }
                        |> SideBar.update (Hover <| Just domID)
                        |> Tuple.second
                        |> List.member (Effects.GetViewportOf domID)
                        |> Expect.true "should check viewport of pipeline"
            , test "does not ask browser for viewport otherwise" <|
                \_ ->
                    { expandedTeams = Set.fromList [ "team" ]
                    , pipelines =
                        RemoteData.Success
                            [ { id = 0
                              , name = "pipeline"
                              , paused = False
                              , public = True
                              , teamName = "team"
                              , groups = []
                              }
                            ]
                    , hovered = Nothing
                    , isSideBarOpen = True
                    , screenSize = ScreenSize.Desktop
                    }
                        |> SideBar.update (Hover <| Just ToggleJobButton)
                        |> Tuple.second
                        |> Expect.equal []
            ]
        ]
