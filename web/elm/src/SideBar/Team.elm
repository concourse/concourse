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
        , name : String
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

        isCurrent =
            (session.currentPipeline |> Maybe.map .teamName) == Just t.name
    in
    { icon =
        if isCurrent then
            Styles.Bright

        else if isHovered || t.isExpanded then
            Styles.GreyedOut

        else
            Styles.Dim
    , collapseIcon =
        { opacity =
            if isCurrent then
                Styles.Bright

            else if t.isExpanded then
                Styles.GreyedOut

            else
                Styles.Dim
        , asset =
            if t.isExpanded then
                Assets.MinusIcon

            else
                Assets.PlusIcon
        }
    , name =
        { text = t.name
        , opacity =
            if isCurrent || isHovered then
                Styles.Bright

            else
                Styles.GreyedOut
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
