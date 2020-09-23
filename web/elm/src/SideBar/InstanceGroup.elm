module SideBar.InstanceGroup exposing (instanceGroup)

import Assets
import Concourse
import HoverState
import Message.Message exposing (DomID(..), Message(..), PipelinesSection(..))
import Routes
import Set exposing (Set)
import SideBar.Styles as Styles
import SideBar.Views as Views


type alias PipelineScoped a =
    { a
        | teamName : String
        , name : String
    }


instanceGroup :
    { a
        | hovered : HoverState.HoverState
        , currentPipeline : Maybe (PipelineScoped b)
        , isFavoritesSection : Bool
    }
    -> Concourse.Pipeline
    -> List Concourse.Pipeline
    -> Views.InstanceGroup
instanceGroup params p ps =
    let
        isCurrent =
            -- TODO: it's also active if we're on the instance group view
            case params.currentPipeline of
                Just cp ->
                    List.any
                        (\pipeline ->
                            cp.name == pipeline.name && cp.teamName == pipeline.teamName
                        )
                        (p :: ps)

                Nothing ->
                    False

        domID =
            SideBarInstanceGroup
                (if params.isFavoritesSection then
                    FavoritesSection

                 else
                    AllPipelinesSection
                )
                p.teamName
                p.name

        isHovered =
            HoverState.isHovered domID params.hovered

        opacity =
            if isCurrent || isHovered then
                Styles.Bright

            else
                Styles.Dim
    in
    { name =
        { opacity = opacity
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
        -- TODO: if we're already on the dashboard, we don't want to lose the query/dashboardView
        Routes.toString <|
            Routes.Dashboard
                { searchType = Routes.Normal "" <| Just { teamName = p.teamName, name = p.name }
                , dashboardView = Routes.ViewNonArchivedPipelines
                }
    , domID = domID
    , badge =
        { count = List.length (p :: ps)
        , opacity = opacity
        }
    }
