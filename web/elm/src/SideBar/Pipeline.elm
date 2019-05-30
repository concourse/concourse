module SideBar.Pipeline exposing (pipeline)

import Concourse
import Html exposing (Html)
import Html.Attributes exposing (href, title)
import Html.Events exposing (onMouseEnter, onMouseLeave)
import Message.Message exposing (DomID(..), Message(..))
import Routes
import SideBar.Styles as Styles


type alias PipelineScoped a =
    { a
        | teamName : String
        , pipelineName : String
    }


pipeline :
    { a
        | hovered : Maybe DomID
        , teamName : String
        , currentPipeline : Maybe (PipelineScoped b)
    }
    -> Concourse.Pipeline
    -> Html Message
pipeline { hovered, teamName, currentPipeline } p =
    let
        pipelineId =
            { pipelineName = p.name
            , teamName = teamName
            }

        isCurrent =
            case currentPipeline of
                Just cp ->
                    cp.pipelineName == p.name && cp.teamName == teamName

                Nothing ->
                    False

        isHovered =
            hovered == Just (SideBarPipeline pipelineId)
    in
    Html.div Styles.pipeline
        [ Html.div
            (Styles.pipelineIcon
                { isCurrent = isCurrent
                , isHovered = isHovered
                }
            )
            []
        , Html.a
            (Styles.pipelineLink
                { isHovered = isHovered
                , isCurrent = isCurrent
                }
                ++ [ href <|
                        Routes.toString <|
                            Routes.Pipeline { id = pipelineId, groups = [] }
                   , title p.name
                   , onMouseEnter <| Hover <| Just <| SideBarPipeline pipelineId
                   , onMouseLeave <| Hover Nothing
                   ]
            )
            [ Html.text p.name ]
        ]
