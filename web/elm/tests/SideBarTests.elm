module SideBarTests exposing (all)

import Browser.Dom
import Expect
import HoverState
import Message.Callback as Callback
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
                    model
                        |> SideBar.update (Hover <| Just domID)
                        |> Tuple.second
                        |> List.member (Effects.GetViewportOf domID)
                        |> Expect.true "should check viewport of pipeline"
            , test "does not ask browser for viewport otherwise" <|
                \_ ->
                    model
                        |> SideBar.update (Hover <| Just ToggleJobButton)
                        |> Tuple.second
                        |> Expect.equal []
            ]
        , describe ".handleCallback"
            [ test "callback with overflowing viewport turns hover -> tooltip" <|
                \_ ->
                    ( { model
                        | hovered =
                            HoverState.Hovered <|
                                SideBarPipeline
                                    { teamName = "team"
                                    , pipelineName = "pipeline"
                                    }
                      }
                    , []
                    )
                        |> SideBar.handleCallback
                            (Callback.GotViewport <| Ok overflowingViewport)
                            RemoteData.NotAsked
                        |> Tuple.first
                        |> .hovered
                        |> Expect.equal
                            (HoverState.Tooltip <|
                                SideBarPipeline
                                    { teamName = "team"
                                    , pipelineName = "pipeline"
                                    }
                            )
            , test "callback with non-overflowing does nothing" <|
                \_ ->
                    ( { model
                        | hovered =
                            HoverState.Hovered <|
                                SideBarPipeline
                                    { teamName = "team"
                                    , pipelineName = "pipeline"
                                    }
                      }
                    , []
                    )
                        |> SideBar.handleCallback
                            (Callback.GotViewport <| Ok nonOverflowingViewport)
                            RemoteData.NotAsked
                        |> Tuple.first
                        |> .hovered
                        |> Expect.equal
                            (HoverState.Hovered <|
                                SideBarPipeline
                                    { teamName = "team"
                                    , pipelineName = "pipeline"
                                    }
                            )
            ]
        ]


overflowingViewport : Browser.Dom.Viewport
overflowingViewport =
    { scene = { width = 1, height = 0 }
    , viewport =
        { width = 0
        , height = 0
        , x = 0
        , y = 0
        }
    }


nonOverflowingViewport : Browser.Dom.Viewport
nonOverflowingViewport =
    { scene = { width = 1, height = 0 }
    , viewport =
        { width = 1
        , height = 0
        , x = 0
        , y = 0
        }
    }


model : SideBar.Model {}
model =
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
    , hovered = HoverState.NoHover
    , isSideBarOpen = True
    , screenSize = ScreenSize.Desktop
    }
