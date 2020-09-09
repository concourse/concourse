module Views.PauseToggle exposing (view)

import Assets
import Html exposing (Html)
import Html.Attributes exposing (class)
import Html.Events exposing (onClick, onMouseEnter, onMouseLeave)
import Message.Message exposing (DomID(..), Message(..), PipelinesSection(..))
import Views.Icon as Icon
import Views.Spinner as Spinner
import Views.Styles as Styles


view :
    { a
        | isPaused : Bool
        , isClickable : Bool
        , isToggleHovered : Bool
        , isToggleLoading : Bool
        , margin : String
        , tooltipPosition : Styles.TooltipPosition
        , domID : DomID
    }
    -> Html Message
view params =
    if params.isToggleLoading then
        Spinner.spinner { sizePx = 20, margin = params.margin }

    else
        Html.div
            (Styles.pauseToggle params.margin
                ++ [ onMouseEnter <| Hover <| Just <| params.domID
                   , onMouseLeave <| Hover Nothing
                   , class "pause-toggle"
                   ]
                ++ (if params.isClickable then
                        [ onClick <| Click <| params.domID ]

                    else
                        []
                   )
            )
            [ Icon.icon
                { sizePx = 20
                , image =
                    if params.isPaused then
                        Assets.PlayIcon

                    else
                        Assets.PauseIcon
                }
                (Styles.pauseToggleIcon
                    { isHovered = params.isClickable && params.isToggleHovered
                    , isClickable = params.isClickable
                    }
                )
            , if params.isToggleHovered && not params.isClickable then
                Html.div
                    (Styles.pauseToggleTooltip params.tooltipPosition)
                    [ Html.text "not authorized" ]

              else
                Html.text ""
            ]
