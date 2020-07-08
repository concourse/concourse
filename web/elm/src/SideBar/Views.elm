module SideBar.Views exposing (Pipeline, Team, viewTeam)

import Assets
import HoverState exposing (TooltipPosition(..))
import Html exposing (Html)
import Html.Attributes exposing (class, href, id)
import Html.Events exposing (onClick, onMouseEnter, onMouseLeave)
import Message.Effects exposing (toHtmlID)
import Message.Message exposing (DomID(..), Message(..))
import SideBar.Styles as Styles


type alias Team =
    { icon : Styles.Opacity
    , collapseIcon :
        { asset : Assets.Asset
        , opacity : Styles.Opacity
        }
    , name :
        { text : String
        , opacity : Styles.Opacity
        , domID : DomID
        }
    , isExpanded : Bool
    , pipelines : List Pipeline
    , background : Styles.Background
    }


viewTeam : Team -> Html Message
viewTeam team =
    Html.div
        (class "side-bar-team" :: Styles.team)
        [ Html.div
            (Styles.teamHeader team
                ++ [ onClick <| Click <| SideBarTeam team.name.text
                   , onMouseEnter <| Hover <| Just <| SideBarTeam team.name.text
                   , onMouseLeave <| Hover Nothing
                   ]
            )
            [ Styles.collapseIcon team.collapseIcon
            , Styles.teamIcon team.icon
            , Html.div
                (Styles.teamName team.name
                    ++ [ id <| toHtmlID team.name.domID ]
                )
                [ Html.text team.name.text ]
            ]
        , if team.isExpanded then
            Html.div Styles.column <| List.map viewPipeline team.pipelines

          else
            Html.text ""
        ]


type alias Pipeline =
    { icon :
        { asset : Assets.Asset
        , opacity : Styles.Opacity
        }
    , name :
        { opacity : Styles.Opacity
        , text : String
        , weight : Styles.FontWeight
        }
    , background : Styles.Background
    , href : String
    , domID : DomID
    }


viewPipeline : Pipeline -> Html Message
viewPipeline p =
    Html.a
        (Styles.pipeline p
            ++ [ href <| p.href
               , onMouseEnter <| Hover <| Just <| p.domID
               , onMouseLeave <| Hover Nothing
               ]
        )
        [ Html.div
            (Styles.pipelineIcon p.icon)
            []
        , Html.div
            (id (toHtmlID p.domID)
                :: Styles.pipelineName p.name
            )
            [ Html.text p.name.text ]
        ]
