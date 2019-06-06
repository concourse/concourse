module SideBar.Views exposing (Pipeline, Team, viewTeam)

import Base64
import Html exposing (Html)
import Html.Attributes exposing (href, id, title)
import Html.Events exposing (onClick, onMouseEnter, onMouseLeave)
import Message.Message exposing (DomID(..), Message(..))
import SideBar.Styles as Styles


type alias Team =
    { icon : Styles.Opacity
    , arrow :
        { icon : Styles.Arrow
        , opacity : Styles.Opacity
        }
    , name :
        { text : String
        , opacity : Styles.Opacity
        , rectangle : Styles.TeamBackingRectangle
        , domID : DomID
        }
    , isExpanded : Bool
    , pipelines : List Pipeline
    }


viewTeam : Team -> Html Message
viewTeam team =
    Html.div
        Styles.team
        [ Html.div
            (Styles.teamHeader
                ++ [ onClick <| Click <| SideBarTeam team.name.text
                   , onMouseEnter <| Hover <| Just <| SideBarTeam team.name.text
                   , onMouseLeave <| Hover Nothing
                   ]
            )
            [ Styles.teamIcon team.icon
            , Styles.arrow team.arrow
            , Html.div
                (Styles.teamName team.name
                    ++ [ title team.name.text
                       , id <| validSideBarId team.name.domID
                       ]
                )
                [ Html.text team.name.text ]
            ]
        , if team.isExpanded then
            Html.div Styles.column <| List.map viewPipeline team.pipelines

          else
            Html.text ""
        ]


type alias Pipeline =
    { icon : Styles.Opacity
    , link :
        { opacity : Styles.Opacity
        , rectangle : Styles.PipelineBackingRectangle
        , text : String
        , href : String
        , domID : DomID
        }
    }


viewPipeline : Pipeline -> Html Message
viewPipeline p =
    Html.div Styles.pipeline
        [ Html.div
            (Styles.pipelineIcon p.icon)
            []
        , Html.a
            (Styles.pipelineLink p.link
                ++ [ href <| p.link.href
                   , title p.link.text
                   , onMouseEnter <| Hover <| Just <| p.link.domID
                   , onMouseLeave <| Hover Nothing
                   , id <| validSideBarId p.link.domID
                   ]
            )
            [ Html.text p.link.text ]
        ]


validSideBarId : DomID -> String
validSideBarId domId =
    case domId of
        SideBarTeam t ->
            "t-" ++ Base64.encode t

        SideBarPipeline p ->
            "p-" ++ Base64.encode p.pipelineName

        _ ->
            ""
