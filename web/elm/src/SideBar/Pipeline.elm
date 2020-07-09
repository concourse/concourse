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
pipeline session p =
    let
        isCurrent =
            case session.currentPipeline of
                Just cp ->
                    cp.pipelineName == p.name && cp.teamName == p.teamName

                Nothing ->
                    False

        pipelineId =
            { pipelineName = p.name, teamName = p.teamName }

        domID =
            SideBarPipeline
                (if session.isFavoritesSection then
                    Favorites

                 else
                    AllPipelines
                )
                pipelineId

        isHovered =
            HoverState.isHovered domID session.hovered
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
    , favIcon =
        { opacity =
            if isCurrent || isHovered then
                Styles.Bright

            else
                Styles.Dim
        , filled = Set.member p.id session.favoritedPipelines
        }
    , id = p.id
    }
