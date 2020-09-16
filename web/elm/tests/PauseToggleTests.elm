module PauseToggleTests exposing (all)

import Data
import Dict
import Message.Message exposing (DomID(..), PipelinesSection(..))
import Test exposing (Test, describe, test)
import Test.Html.Query as Query
import Test.Html.Selector exposing (containing, style, tag, text)
import UserState exposing (UserState(..))
import Views.PauseToggle as PauseToggle
import Views.Styles as Styles


all : Test
all =
    describe "pause toggle"
        [ describe "when user is unauthorized" <|
            let
                pipeline =
                    Data.pipelineId

                userState =
                    UserStateLoggedIn
                        { id = "test"
                        , userName = "test"
                        , name = "test"
                        , email = "test"
                        , isAdmin = False
                        , teams = Dict.fromList [ ( "team", [ "viewer" ] ) ]
                        }
            in
            [ test "has very low opacity" <|
                \_ ->
                    PauseToggle.view
                        { isPaused = False
                        , pipeline = pipeline
                        , isToggleHovered = False
                        , isToggleLoading = False
                        , margin = ""
                        , userState = userState
                        , tooltipPosition = Styles.Above
                        , domID = PipelineCardPauseToggle AllPipelinesSection pipeline
                        }
                        |> Query.fromHtml
                        |> Query.has [ style "opacity" "0.2" ]
            , test "has tooltip above" <|
                \_ ->
                    PauseToggle.view
                        { isPaused = False
                        , pipeline = pipeline
                        , isToggleHovered = True
                        , isToggleLoading = False
                        , margin = ""
                        , userState = userState
                        , tooltipPosition = Styles.Above
                        , domID = PipelineCardPauseToggle AllPipelinesSection pipeline
                        }
                        |> Query.fromHtml
                        |> Query.has
                            [ style "position" "relative"
                            , containing
                                [ tag "div"
                                , containing [ text "not authorized" ]
                                , style "background-color" "#9b9b9b"
                                , style "position" "absolute"
                                , style "bottom" "100%"
                                , style "white-space" "nowrap"
                                , style "padding" "2.5px"
                                , style "margin-bottom" "5px"
                                , style "right" "-150%"
                                , style "z-index" "1"
                                ]
                            ]
            , test "has tooltip below" <|
                \_ ->
                    PauseToggle.view
                        { isPaused = False
                        , pipeline = pipeline
                        , isToggleHovered = True
                        , isToggleLoading = False
                        , margin = ""
                        , userState = userState
                        , tooltipPosition = Styles.Below
                        , domID = PipelineCardPauseToggle AllPipelinesSection pipeline
                        }
                        |> Query.fromHtml
                        |> Query.has
                            [ style "position" "relative"
                            , containing
                                [ tag "div"
                                , containing [ text "not authorized" ]
                                , style "background-color" "#9b9b9b"
                                , style "position" "absolute"
                                , style "top" "100%"
                                , style "white-space" "nowrap"
                                , style "padding" "2.5px"
                                , style "margin-bottom" "5px"
                                , style "right" "-150%"
                                , style "z-index" "1"
                                ]
                            ]
            ]
        ]
