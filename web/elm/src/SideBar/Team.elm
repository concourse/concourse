module SideBar.Team exposing (team)

import Assets
import Concourse
import HoverState
import Message.Message exposing (DomID(..), Message(..), PipelinesSection(..))
import Set exposing (Set)
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
        , pipelines : List Concourse.Pipeline
        , currentPipeline : Maybe (PipelineScoped b)
        , favoritedPipelines : Set Int
        , isFavoritesSection : Bool
    }
    -> { name : String, isExpanded : Bool }
    -> Views.Team
team session t =
    let
        domID =
            SideBarTeam
                (if session.isFavoritesSection then
                    FavoritesSection

                 else
                    AllPipelinesSection
                )
                t.name

        isHovered =
            HoverState.isHovered domID session.hovered
    in
    { icon =
        if isHovered then
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
        , teamColor =
            if isHovered then
                Styles.White

            else
                Styles.LightGrey
        , domID = domID
        }
    , isExpanded = t.isExpanded
    , pipelines = List.map (Pipeline.pipeline session) session.pipelines
    , background =
        if isHovered then
            Styles.Light

        else
            Styles.Invisible
    }
