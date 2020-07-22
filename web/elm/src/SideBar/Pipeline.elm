module SideBar.Pipeline exposing (pipeline)

import Assets
import Concourse
import HoverState
import Message.Message exposing (DomID(..), Message(..), SideBarSection(..))
import Routes
import Set exposing (Set)
import SideBar.Styles as Styles
import SideBar.Views as Views


type alias PipelineScoped a =
    { a
        | teamName : String
        , pipelineName : String
    }


pipeline :
    { a
        | hovered : HoverState.HoverState
        , currentPipeline : Maybe (PipelineScoped b)
        , favoritedPipelines : Set Int
        , isFavoritesSection : Bool
    }
    -> Concourse.Pipeline
    -> Views.Pipeline
pipeline params p =
    let
        isCurrent =
            case params.currentPipeline of
                Just cp ->
                    cp.pipelineName == p.name && cp.teamName == p.teamName

                Nothing ->
                    False

        pipelineId =
            { pipelineName = p.name, teamName = p.teamName }

        domID =
            SideBarPipeline
                (if params.isFavoritesSection then
                    Favorites

                 else
                    AllPipelines
                )
                pipelineId

        isHovered =
            HoverState.isHovered domID params.hovered
    in
    { icon =
        { asset =
            if p.archived then
                Assets.ArchivedPipelineIcon

            else
                Assets.BreadcrumbIcon Assets.PipelineComponent
        , opacity =
            if isCurrent || isHovered then
                Styles.Bright

            else
                Styles.Dim
        }
    , name =
        { opacity =
            if isCurrent || isHovered then
                Styles.Bright

            else
                Styles.Dim
        , text = p.name
        , weight =
            if isCurrent then
                Styles.Bold

            else
                Styles.Default
        }
    , background =
        if isCurrent then
            Styles.Dark

        else if isHovered then
            Styles.Light

        else
            Styles.Invisible
    , href =
        Routes.toString <|
            Routes.Pipeline { id = pipelineId, groups = [] }
    , domID = domID
    , starIcon =
        { opacity =
            if isCurrent || isHovered then
                Styles.Bright

            else
                Styles.Dim
        , filled = Set.member p.id params.favoritedPipelines
        }
    , id = p.id
    }
