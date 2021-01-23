module SideBar.Team exposing (team)

import Assets
import Concourse exposing (PipelineGrouping(..))
import HoverState
import Message.Message exposing (DomID(..), Message(..), PipelinesSection(..))
import Set exposing (Set)
import SideBar.InstanceGroup as InstanceGroup
import SideBar.Pipeline as Pipeline
import SideBar.Styles as Styles
import SideBar.Views as Views


type alias PipelineScoped a =
    { a
        | teamName : String
        , pipelineName : String
    }


team :
    { a
        | hovered : HoverState.HoverState
        , pipelines : List (PipelineGrouping Concourse.Pipeline)
        , currentPipeline : Maybe (PipelineScoped b)
        , favoritedPipelines : Set Int
        , favoritedInstanceGroups : Set ( Concourse.TeamName, Concourse.PipelineName )
        , isFavoritesSection : Bool
    }
    -> { name : String, isExpanded : Bool }
    -> Views.Team
team params t =
    let
        domID =
            SideBarTeam
                (if params.isFavoritesSection then
                    FavoritesSection

                 else
                    AllPipelinesSection
                )
                t.name

        isHovered =
            HoverState.isHovered domID params.hovered

        isCurrent =
            (params.currentPipeline |> Maybe.map .teamName) == Just t.name
    in
    { icon =
        if isHovered || isCurrent then
            Styles.Bright

        else
            Styles.GreyedOut
    , collapseIcon =
        { opacity =
            Styles.Bright
        , asset =
            if t.isExpanded then
                Assets.MinusIcon

            else
                Assets.PlusIcon
        }
    , name =
        { text = t.name
        , color =
            if isHovered || isCurrent then
                Styles.White

            else
                Styles.LightGrey
        , domID = domID
        }
    , isExpanded = t.isExpanded
    , listItems =
        params.pipelines
            |> List.map
                (\g ->
                    case g of
                        Concourse.RegularPipeline p ->
                            Pipeline.pipeline params p |> Views.PipelineListItem

                        Concourse.InstanceGroup p ps ->
                            InstanceGroup.instanceGroup params p ps |> Views.InstanceGroupListItem
                )
    , background =
        if isHovered then
            Styles.Light

        else
            Styles.Invisible
    }
