module SideBar.Views exposing
    ( Icon(..)
    , InstanceGroup
    , Pipeline
    , Team
    , TeamListItem(..)
    , viewTeam
    )

import Assets
import Concourse
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
        , color : Styles.SidebarElementColor
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
            , Styles.teamIcon
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


type Icon
    = AssetIcon Assets.Asset
    | TextIcon String


type alias Pipeline =
    { icon : Icon
    , name :
        { color : Styles.SidebarElementColor
        , text : String
        , weight : Styles.FontWeight
        }
    , background : Styles.Background
    , href : String
    , domID : DomID
    , starIcon :
        { filled : Bool
        , isBright : Bool
        }
    , id : Concourse.PipelineIdentifier
    , databaseID : Concourse.DatabaseID
    }


type alias InstanceGroup =
    { name :
        { color : Styles.SidebarElementColor
        , text : String
        , weight : Styles.FontWeight
        }
    , background : Styles.Background
    , href : String
    , domID : DomID
    , badge :
        { count : Int
        , color : Styles.SidebarElementColor
        }
    , starIcon :
        { filled : Bool
        , isBright : Bool
        }
    , id : Concourse.InstanceGroupIdentifier
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
        [ case p.icon of
            AssetIcon asset ->
                Html.div
                    (Styles.pipelineIcon asset)
                    []

            TextIcon s ->
                Html.div
                    Styles.pipelineTextIcon
                    [ Html.text s ]
        , Html.div
            (id (toHtmlID p.domID)
                :: Styles.pipelineName p.name
            )
            [ Html.text p.name.text ]
        , Html.div
            (Styles.favoriteIcon p.starIcon
                ++ [ onLeftClickStopPropagation <| Click <| SideBarPipelineFavoritedIcon p.databaseID ]
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
        , Html.div
            (Styles.favoriteIcon ig.starIcon
                ++ [ onLeftClickStopPropagation <| Click <| SideBarInstanceGroupFavoritedIcon ig.id ]
            )
            []
        ]


viewListItem : TeamListItem -> Html Message
viewListItem item =
    case item of
        PipelineListItem p ->
            viewPipeline p

        InstanceGroupListItem ps ->
            viewInstanceGroup ps
