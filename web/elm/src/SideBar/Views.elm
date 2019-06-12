module SideBar.Views exposing (Pipeline, Team, viewTeam)

import Colors
import HoverState exposing (TooltipPosition)
import Html exposing (Html)
import Html.Attributes exposing (href, id)
import Html.Events exposing (onClick, onMouseEnter, onMouseLeave)
import Message.Effects exposing (toHtmlID)
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
        , tooltip : Maybe TooltipPosition
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
                    ++ [ id <| toHtmlID team.name.domID ]
                )
                [ Html.text team.name.text ]
            , tooltip team.name.tooltip team.name.text
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
        , tooltip : Maybe TooltipPosition
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
                   , onMouseEnter <| Hover <| Just <| p.link.domID
                   , onMouseLeave <| Hover Nothing
                   , id <| toHtmlID p.link.domID
                   ]
            )
            [ Html.text p.link.text
            , tooltip p.link.tooltip p.link.text
            ]
        ]


tooltip : Maybe TooltipPosition -> String -> Html Message
tooltip tp text =
    case tp of
        Nothing ->
            Html.text ""

        Just { x, y } ->
            Html.div
                [ Html.Attributes.style "position" "fixed"
                , Html.Attributes.style "left" <| String.fromFloat x ++ "px"
                , Html.Attributes.style "top" <| String.fromFloat y ++ "px"
                , Html.Attributes.style "pointer-events" "none"
                , Html.Attributes.style "z-index" "1"
                , Html.Attributes.style "display" "flex"
                ]
                [ Html.div
                    [ Html.Attributes.style "border-right" <| "15px solid " ++ Colors.frame
                    , Html.Attributes.style "border-top" "15px solid transparent"
                    , Html.Attributes.style "border-bottom" "15px solid transparent"
                    ]
                    []
                , Html.div
                    [ Html.Attributes.style "background-color" Colors.frame
                    , Html.Attributes.style "line-height" "28px"
                    , Html.Attributes.style "padding-right" "10px"
                    , Html.Attributes.style "font-size" "12px"
                    ]
                    [ Html.text text ]
                ]
