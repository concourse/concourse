module SideBar.Views exposing
    ( InstanceGroup
    , Pipeline
    , Team
    , TeamListItem(..)
    , viewTeam
    )

import Assets
import HoverState exposing (TooltipPosition(..))
import Html exposing (Html)
import Html.Attributes exposing (class, href, id)
import Html.Events exposing (onClick, onMouseEnter, onMouseLeave)
import Message.Effects exposing (toHtmlID)
import Message.Message exposing (DomID(..), Message(..))
import SideBar.Styles as Styles
import StrictEvents exposing (onLeftClickStopPropagation)


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
    , listItems : List TeamListItem
    , background : Styles.Background
    }


viewTeam : Team -> Html Message
viewTeam team =
    Html.div
        (class "side-bar-team" :: Styles.team)
        [ Html.div
            (Styles.teamHeader team
                ++ [ onClick <| Click <| team.name.domID
                   , onMouseEnter <| Hover <| Just <| team.name.domID
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
            Html.div Styles.column <| List.map viewListItem team.listItems

          else
            Html.text ""
        ]


type TeamListItem
    = PipelineListItem Pipeline
    | InstanceGroupListItem InstanceGroup


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
    , starIcon :
        { opacity : Styles.Opacity
        , filled : Bool
        }
    , id : Int
    }


type alias InstanceGroup =
    { name :
        { opacity : Styles.Opacity
        , text : String
        , weight : Styles.FontWeight
        }
    , background : Styles.Background
    , href : String
    , domID : DomID
    , badge :
        { count : Int
        , opacity : Styles.Opacity
        }
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
        , Html.div
            (Styles.pipelineFavourite p.starIcon
                ++ [ onLeftClickStopPropagation <| Click <| SideBarFavoritedIcon p.id ]
            )
            []
        ]


viewInstanceGroup : InstanceGroup -> Html Message
viewInstanceGroup ig =
    Html.a
        (Styles.instanceGroup ig
            ++ [ href <| ig.href
               , onMouseEnter <| Hover <| Just <| ig.domID
               , onMouseLeave <| Hover Nothing
               ]
        )
        [ Styles.instanceGroupBadge ig.badge
        , Html.div
            (id (toHtmlID ig.domID)
                :: Styles.pipelineName ig.name
            )
            [ Html.text ig.name.text ]
        ]


viewListItem : TeamListItem -> Html Message
viewListItem item =
    case item of
        PipelineListItem p ->
            viewPipeline p

        InstanceGroupListItem ps ->
            viewInstanceGroup ps
