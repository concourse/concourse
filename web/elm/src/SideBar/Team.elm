module SideBar.Team exposing (team)

import Concourse
import Html exposing (Html)
import Html.Attributes exposing (title)
import Html.Events exposing (onClick, onMouseEnter, onMouseLeave)
import Message.Message exposing (DomID(..), Message(..))
import SideBar.Pipeline as Pipeline
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
            Html.div Styles.column <| List.map (Pipeline.pipeline session) pipelines

          else
            Html.text ""
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
        [ Styles.teamIcon { isExpanded = isExpanded, isCurrent = isCurrent, isHovered = isHovered }
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
