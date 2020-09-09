module PinMenu.PinMenuTests exposing (all)

import Colors
import Concourse
import Data
import Dict
import Expect
import HoverState
import Json.Encode as JE
import Message.Message exposing (DomID(..), Message(..))
import Pipeline.PinMenu.PinMenu as PinMenu
import Pipeline.PinMenu.Views as Views
import Pipeline.Pipeline as Pipeline
import Routes
import SideBar.Styles as SS
import Test exposing (Test, describe, test)
import Test.Html.Event as Event
import Test.Html.Query as Query
import Test.Html.Selector exposing (id)
import Views.Styles


init =
    Pipeline.init
        { pipelineLocator = Data.pipelineId
        , turbulenceImgSrc = ""
        , selectedGroups = []
        }
        |> Tuple.first


all : Test
all =
    describe "pin menu"
        [ test "not clickable if there are no pinned resources" <|
            \_ ->
                init
                    |> PinMenu.pinMenu { hovered = HoverState.NoHover }
                    |> .clickable
                    |> Expect.equal False
        , test "has dim icon if there are no pinned resources" <|
            \_ ->
                init
                    |> PinMenu.pinMenu { hovered = HoverState.NoHover }
                    |> .opacity
                    |> Expect.equal SS.Dim
        , test "has dark background" <|
            \_ ->
                init
                    |> PinMenu.pinMenu { hovered = HoverState.NoHover }
                    |> .background
                    |> Expect.equal Views.Dark
        , test "has no badge if there are no pinned resources" <|
            \_ ->
                init
                    |> PinMenu.pinMenu { hovered = HoverState.NoHover }
                    |> .badge
                    |> Expect.equal Nothing
        , describe "with pinned resources" <|
            let
                model =
                    { init
                        | fetchedResources =
                            Just <|
                                JE.list identity
                                    [ JE.object
                                        [ ( "team_name", JE.string "team" )
                                        , ( "pipeline_name", JE.string "pipeline" )
                                        , ( "pipeline_id", JE.int 1 )
                                        , ( "name", JE.string "test" )
                                        , ( "type", JE.string "type" )
                                        , ( "pinned_version"
                                          , JE.object
                                                [ ( "version"
                                                  , JE.string "v1"
                                                  )
                                                ]
                                          )
                                        ]
                                    ]
                    }
            in
            [ test "is hoverable" <|
                \_ ->
                    model
                        |> PinMenu.pinMenu { hovered = HoverState.NoHover }
                        |> .hoverable
                        |> Expect.equal True
            , test "is clickable" <|
                \_ ->
                    model
                        |> PinMenu.pinMenu { hovered = HoverState.NoHover }
                        |> .clickable
                        |> Expect.equal True
            , test "has greyed-out icon" <|
                \_ ->
                    model
                        |> PinMenu.pinMenu { hovered = HoverState.NoHover }
                        |> .opacity
                        |> Expect.equal SS.GreyedOut
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
                                        (Views.Px 10)
                                        (Views.Px 10)
                                , text = "1"
                                }
                            )
            , test "has no dropdown when unhovered" <|
                \_ ->
                    model
                        |> PinMenu.pinMenu { hovered = HoverState.NoHover }
                        |> .dropdown
                        |> Expect.equal Nothing
            , test "has bright icon when hovered" <|
                \_ ->
                    model
                        |> PinMenu.pinMenu { hovered = HoverState.Hovered PinIcon }
                        |> .opacity
                        |> Expect.equal SS.Bright
            , test "clicking brightens background" <|
                \_ ->
                    ( model, [] )
                        |> PinMenu.update (Click PinIcon)
                        |> Tuple.first
                        |> PinMenu.pinMenu { hovered = HoverState.NoHover }
                        |> .background
                        |> Expect.equal Views.Light
            , test "clicking brightens icon" <|
                \_ ->
                    ( model, [] )
                        |> PinMenu.update (Click PinIcon)
                        |> Tuple.first
                        |> PinMenu.pinMenu { hovered = HoverState.NoHover }
                        |> .opacity
                        |> Expect.equal SS.Bright
            , test "clicking reveals dropdown" <|
                \_ ->
                    ( model, [] )
                        |> PinMenu.update (Click PinIcon)
                        |> Tuple.first
                        |> PinMenu.pinMenu { hovered = HoverState.NoHover }
                        |> .dropdown
                        |> Expect.equal
                            (Just
                                { position =
                                    Views.TopRight
                                        (Views.Percent 100)
                                        (Views.Percent 0)
                                , items =
                                    [ { title =
                                            { content = "test"
                                            , fontWeight = Views.Styles.fontWeightDefault
                                            , color = Colors.text
                                            }
                                      , table =
                                            [ { left = "version"
                                              , right = "v1"
                                              , color = Colors.text
                                              }
                                            ]
                                      , background = Colors.sideBar
                                      , paddingPx = 10
                                      , hoverable = True
                                      , onClick =
                                            GoToRoute <|
                                                Routes.Resource
                                                    { id = Data.resourceId |> Data.withResourceName "test"
                                                    , page = Nothing
                                                    }
                                      }
                                    ]
                                }
                            )
            , test "clicking again dismisses dropdown" <|
                \_ ->
                    ( { model | pinMenuExpanded = True }, [] )
                        |> PinMenu.update (Click PinIcon)
                        |> Tuple.first
                        |> PinMenu.pinMenu { hovered = HoverState.NoHover }
                        |> .dropdown
                        |> Expect.equal Nothing
            , test "hovered dropdown item has darker background" <|
                \_ ->
                    { model | pinMenuExpanded = True }
                        |> PinMenu.pinMenu
                            { hovered =
                                HoverState.Hovered <| PinMenuDropDown "test"
                            }
                        |> .dropdown
                        |> Expect.equal
                            (Just
                                { position =
                                    Views.TopRight
                                        (Views.Percent 100)
                                        (Views.Percent 0)
                                , items =
                                    [ { title =
                                            { content = "test"
                                            , fontWeight = Views.Styles.fontWeightDefault
                                            , color = Colors.text
                                            }
                                      , table =
                                            [ { left = "version"
                                              , right = "v1"
                                              , color = Colors.text
                                              }
                                            ]
                                      , background = Colors.sideBarActive
                                      , paddingPx = 10
                                      , hoverable = True
                                      , onClick =
                                            GoToRoute <|
                                                Routes.Resource
                                                    { id = Data.resourceId |> Data.withResourceName "test"
                                                    , page = Nothing
                                                    }
                                      }
                                    ]
                                }
                            )
            ]
        ]
