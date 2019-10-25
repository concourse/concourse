module PinMenu.PinMenuTests exposing (all)

import Colors
import Concourse
import Dict
import Expect
import HoverState
import Message.Message exposing (DomID(..), Message(..))
import Pipeline.PinMenu.PinMenu as PinMenu
import Pipeline.PinMenu.Views as Views
import Routes
import Test exposing (Test, describe, test)
import Test.Html.Event as Event
import Test.Html.Query as Query
import Test.Html.Selector exposing (id)


all : Test
all =
    describe "pin menu"
        [ test "not hoverable if there are no pinned resources" <|
            \_ ->
                { pinnedResources = []
                , pipeline = { pipelineName = "pipeline", teamName = "team" }
                }
                    |> PinMenu.pinMenu { hovered = HoverState.NoHover }
                    |> .hoverable
                    |> Expect.equal False
        , test "has dim icon if there are no pinned resources" <|
            \_ ->
                { pinnedResources = []
                , pipeline = { pipelineName = "pipeline", teamName = "team" }
                }
                    |> PinMenu.pinMenu { hovered = HoverState.NoHover }
                    |> .iconStyle
                    |> Expect.equal Views.Dim
        , test "has dark background when unhovered" <|
            \_ ->
                { pinnedResources = []
                , pipeline = { pipelineName = "pipeline", teamName = "team" }
                }
                    |> PinMenu.pinMenu { hovered = HoverState.NoHover }
                    |> .background
                    |> Expect.equal Views.Dark
        , test "has no badge if there are no pinned resources" <|
            \_ ->
                { pinnedResources = []
                , pipeline = { pipelineName = "pipeline", teamName = "team" }
                }
                    |> PinMenu.pinMenu { hovered = HoverState.NoHover }
                    |> .badge
                    |> Expect.equal Nothing
        , describe "with pinned resources" <|
            let
                model =
                    { pinnedResources =
                        [ ( "test"
                          , Dict.fromList [ ( "version", "v1" ) ]
                          )
                        ]
                    , pipeline =
                        { pipelineName = "pipeline"
                        , teamName = "team"
                        }
                    }
            in
            [ test "is hoverable" <|
                \_ ->
                    model
                        |> PinMenu.pinMenu { hovered = HoverState.NoHover }
                        |> .hoverable
                        |> Expect.equal True
            , test "has dark background" <|
                \_ ->
                    model
                        |> PinMenu.pinMenu { hovered = HoverState.NoHover }
                        |> .background
                        |> Expect.equal Views.Dark
            , test "has pin count badge" <|
                \_ ->
                    model
                        |> PinMenu.pinMenu { hovered = HoverState.NoHover }
                        |> .badge
                        |> Expect.equal
                            (Just
                                { color = Colors.pinned
                                , diameterPx = 15
                                , position =
                                    Views.TopRight
                                        (Views.Px 3)
                                        (Views.Px 3)
                                , text = "1"
                                }
                            )
            , test "has no dropdown when unhovered" <|
                \_ ->
                    model
                        |> PinMenu.pinMenu { hovered = HoverState.NoHover }
                        |> .dropdown
                        |> Expect.equal Nothing
            , test "hovering changes background to spotlight" <|
                \_ ->
                    model
                        |> PinMenu.pinMenu { hovered = HoverState.Hovered PinIcon }
                        |> .background
                        |> Expect.equal Views.Spotlight
            , test "hovering reveals dropdown" <|
                \_ ->
                    model
                        |> PinMenu.pinMenu { hovered = HoverState.Hovered PinIcon }
                        |> .dropdown
                        |> Expect.equal
                            (Just
                                { background = Colors.white
                                , position =
                                    Views.TopRight
                                        (Views.Percent 100)
                                        (Views.Percent 0)
                                , paddingPx = 10
                                , items =
                                    [ { title =
                                            { content = "test"
                                            , fontWeight = 700
                                            , color = Colors.frame
                                            }
                                      , table = [ PinMenu.TableRow "version" "v1" ]
                                      , onClick =
                                            GoToRoute <|
                                                Routes.Resource
                                                    { id =
                                                        { teamName = "team"
                                                        , pipelineName = "pipeline"
                                                        , resourceName = "test"
                                                        }
                                                    , page = Nothing
                                                    }
                                      }
                                    ]
                                }
                            )
            ]
        ]
