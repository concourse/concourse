module Views.PauseToggle exposing (view)

import Concourse
import Html exposing (Html)
import Html.Attributes exposing (id, style)
import Html.Events exposing (onClick, onMouseEnter, onMouseLeave)
import Message.Message exposing (Hoverable(..), Message(..))
import TopBar.Styles as Styles
import UserState exposing (UserState(..))
import Views.Icon as Icon
import Views.Spinner as Spinner


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
        Spinner.spinner { size = "20px", margin = margin }

    else
        Icon.icon
            { sizePx = 20
            , image =
                if isPaused then
                    "ic-play-white.svg"

                else
                    "ic-pause-white.svg"
            }
            ([ style <|
                Styles.pauseToggleIcon
                    { isHovered = isClickable && isToggleHovered
                    , isClickable = isClickable
                    , margin = margin
                    }
             , onMouseEnter <| Hover <| Just <| PipelineButton pipeline
             , onMouseLeave <| Hover Nothing
             ]
                ++ (if isClickable then
                        [ onClick <| TogglePipelinePaused pipeline isPaused ]

                    else
                        []
                   )
            )
