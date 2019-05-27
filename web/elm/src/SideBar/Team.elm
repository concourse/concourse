module SideBar.Team exposing (team)

import Concourse
import Html exposing (Html)
import Html.Attributes exposing (href, id, title)
import Html.Events exposing (onClick, onMouseEnter, onMouseLeave)
import Message.Message exposing (DomID(..), Message(..))
import Routes
import SideBar.Styles as Styles


type alias PipelineScoped a =
    { a
        | teamName : String
        , pipelineName : String
    }


team :
    { a
        | hovered : Maybe DomID
        , isExpanded : Bool
        , teamName : String
        , pipelines : List Concourse.Pipeline
        , currentPipeline : Maybe (PipelineScoped b)
    }
    -> Html Message
team ({ isExpanded, pipelines } as session) =
    Html.div
        Styles.team
        [ teamHeader session
        , if isExpanded then
            Html.div Styles.column <| List.map (pipeline session) pipelines

          else
            Html.text ""
        ]


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


teamHeader :
    { a
        | hovered : Maybe DomID
        , isExpanded : Bool
        , teamName : String
        , currentPipeline : Maybe (PipelineScoped b)
    }
    -> Html Message
teamHeader { hovered, isExpanded, teamName, currentPipeline } =
    let
        isHovered =
            hovered == Just (SideBarTeam teamName)

        isCurrent =
            (currentPipeline
                |> Maybe.map .teamName
            )
                == Just teamName
    in
    Html.div
        (Styles.teamHeader
            ++ [ onClick <| Click <| SideBarTeam teamName
               , onMouseEnter <| Hover <| Just <| SideBarTeam teamName
               , onMouseLeave <| Hover Nothing
               ]
        )
        [ Styles.teamIcon { isCurrent = isCurrent, isHovered = isHovered }
        , Styles.arrow
            { isHovered = isHovered
            , isExpanded = isExpanded
            }
        , Html.div
            (title teamName
                :: Styles.teamName
                    { isHovered = isHovered
                    , isCurrent = isCurrent
                    }
            )
            [ Html.text teamName ]
        ]
