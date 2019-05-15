module Views.PauseToggle exposing (view)

import Concourse
import Html exposing (Html)
import Html.Attributes exposing (class)
import Html.Events exposing (onClick, onMouseEnter, onMouseLeave)
import Message.Message exposing (DomID(..), Message(..))
import UserState exposing (UserState(..))
import Views.Icon as Icon
import Views.Spinner as Spinner
import Views.Styles as Styles


view :
    { a
        | isPaused : Bool
        , pipeline : Concourse.PipelineIdentifier
        , isToggleHovered : Bool
        , isToggleLoading : Bool
        , margin : String
        , userState : UserState
        , tooltipPosition : Styles.TooltipPosition
    }
    -> Html Message
view params =
    let
        isClickable =
            UserState.isAnonymous params.userState
                || UserState.isMember
                    { teamName = params.pipeline.teamName
                    , userState = params.userState
                    }
    in
    if params.isToggleLoading then
        Spinner.spinner { sizePx = 20, margin = params.margin }

    else
        Html.div
            (Styles.pauseToggle params.margin
                ++ [ onMouseEnter <| Hover <| Just <| PipelineButton params.pipeline
                   , onMouseLeave <| Hover Nothing
                   , class "pause-toggle"
                   ]
                ++ (if isClickable then
                        [ onClick <| Click <| PipelineButton params.pipeline ]

                    else
                        []
                   )
            )
            [ Icon.icon
                { sizePx = 20
                , image =
                    if params.isPaused then
                        "ic-play-white.svg"

                    else
                        "ic-pause-white.svg"
                }
                (Styles.pauseToggleIcon
                    { isHovered = isClickable && params.isToggleHovered
                    , isClickable = isClickable
                    }
                )
            , if params.isToggleHovered && not isClickable then
                Html.div
                    (Styles.pauseToggleTooltip params.tooltipPosition)
                    [ Html.text "not authorized" ]

              else
                Html.text ""
            ]
