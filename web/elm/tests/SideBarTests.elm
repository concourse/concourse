module SideBarTests exposing (all)

import Browser.Dom
import Common
import Expect
import HoverState
import Message.Callback as Callback exposing (TooltipPolicy(..))
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
                    model
                        |> SideBar.update (Hover <| Just domID)
                        |> Tuple.second
                        |> Common.contains
                            (Effects.GetViewportOf domID OnlyShowWhenOverflowing)
            , test "does not ask browser for viewport otherwise" <|
                \_ ->
                    model
                        |> SideBar.update (Hover <| Just ToggleJobButton)
                        |> Tuple.second
                        |> Expect.equal []
            ]
        ]


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


domID : DomID
domID =
    SideBarPipeline
        { teamName = "team"
        , pipelineName = "pipeline"
        }
