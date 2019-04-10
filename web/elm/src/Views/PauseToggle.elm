module Views.PauseToggle exposing (view)

import Concourse
import Html exposing (Html)
import Html.Events exposing (onClick, onMouseEnter, onMouseLeave)
import Message.Message exposing (DomID(..), Message(..))
import UserState exposing (UserState(..))
import Views.Icon as Icon
import Views.Spinner as Spinner
import Views.Styles as Styles


view :
    String
    -> UserState
    ->
        { a
            | isPaused : Bool
            , pipeline : Concourse.PipelineIdentifier
            , isToggleHovered : Bool
            , isToggleLoading : Bool
        }
    -> Html Message
view margin userState { isPaused, pipeline, isToggleHovered, isToggleLoading } =
    let
        isClickable =
            UserState.isAnonymous userState
                || UserState.isMember
                    { teamName = pipeline.teamName
                    , userState = userState
                    }
    in
    if isToggleLoading then
        Spinner.spinner { sizePx = 20, margin = margin }

    else
        Icon.icon
            { sizePx = 20
            , image =
                if isPaused then
                    "ic-play-white.svg"

                else
                    "ic-pause-white.svg"
            }
            ([ onMouseEnter <| Hover <| Just <| PipelineButton pipeline
             , onMouseLeave <| Hover Nothing
             ]
                ++ Styles.pauseToggleIcon
                    { isHovered = isClickable && isToggleHovered
                    , isClickable = isClickable
                    , margin = margin
                    }
                ++ (if isClickable then
                        [ onClick <| Click <| PipelineButton pipeline ]

                    else
                        []
                   )
            )
