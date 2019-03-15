module PauseToggle exposing (view)

import Html exposing (Html)
import Html.Attributes exposing (id, style)
import Html.Events exposing (onClick, onMouseEnter, onMouseLeave)
import Spinner
import TopBar.Model exposing (PipelineState)
import TopBar.Msgs exposing (Msg(..))
import TopBar.Styles as Styles
import UserState exposing (UserState(..))


view :
    UserState
    -> Maybe PipelineState
    -> Html Msg
view userState pipelineState =
    case pipelineState of
        Just { isPaused, pipeline, isToggleHovered, isToggleLoading } ->
            let
                isAnonymous =
                    UserState.user userState == Nothing

                isMember =
                    UserState.isMember
                        { teamName = pipeline.teamName
                        , userState = userState
                        }

                isClickable =
                    isAnonymous || isMember
            in
            Html.div
                ([ id "top-bar-pause-toggle"
                 , style <|
                    Styles.pauseToggle
                        { isPaused = isPaused
                        , isClickable = isClickable
                        }
                 , onMouseEnter <| Hover True
                 , onMouseLeave <| Hover False
                 ]
                    ++ (if isClickable then
                            [ onClick <| TogglePipelinePaused pipeline isPaused ]

                        else
                            []
                       )
                )
                [ if isToggleLoading then
                    Spinner.spinner { size = "20px", margin = "0" }

                  else
                    Html.div
                        [ style <|
                            Styles.pauseToggleIcon
                                { isPaused = isPaused
                                , isHovered = isClickable && isToggleHovered
                                }
                        ]
                        []
                ]

        Nothing ->
            Html.text ""
