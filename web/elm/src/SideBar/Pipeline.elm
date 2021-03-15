module SideBar.Pipeline exposing (instancedPipeline, instancedPipelineText, regularPipeline, regularPipelineText)

import Assets
import Concourse
import Dict
import Favorites
import HoverState
import Message.Message exposing (DomID(..), Message(..), PipelinesSection(..))
import Routes
import Set exposing (Set)
import SideBar.Styles as Styles
import SideBar.Views as Views


type alias PipelineScoped a =
    { a
        | teamName : String
        , pipelineName : String
        , pipelineInstanceVars : Concourse.InstanceVars
    }


type alias Params a b =
    { a
        | hovered : HoverState.HoverState
        , currentPipeline : Maybe (PipelineScoped b)
        , favoritedPipelines : Set Int
        , isFavoritesSection : Bool
    }


pipeline : Bool -> Params a b -> Concourse.Pipeline -> Views.Pipeline
pipeline isInstancedPipeline params p =
    let
        isCurrent =
            case params.currentPipeline of
                Just cp ->
                    (cp.pipelineName == p.name)
                        && (cp.teamName == p.teamName)
                        && (cp.pipelineInstanceVars == p.instanceVars)

                Nothing ->
                    False

        pipelineId =
            Concourse.toPipelineId p

        domIDFn =
            if isInstancedPipeline then
                SideBarInstancedPipeline

            else
                SideBarPipeline

        domID =
            domIDFn
                (if params.isFavoritesSection then
                    FavoritesSection

                 else
                    AllPipelinesSection
                )
                p.id

        isHovered =
            HoverState.isHovered domID params.hovered

        isFavorited =
            Favorites.isPipelineFavorited params p
    in
    { icon =
        if p.archived then
            Views.AssetIcon Assets.ArchivedPipelineIcon

        else if isInstancedPipeline then
            Views.TextIcon "/"

        else
            Views.AssetIcon <|
                if isHovered then
                    Assets.PipelineIconWhite

                else if isCurrent then
                    Assets.PipelineIconLightGrey

                else
                    Assets.PipelineIconGrey
    , name =
        { color =
            if isHovered then
                Styles.White

            else if isCurrent then
                Styles.LightGrey

            else
                Styles.Grey
        , text =
            if isInstancedPipeline then
                instancedPipelineText p

            else
                regularPipelineText p
        , weight =
            if isCurrent || isHovered then
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
        { filled = isFavorited
        , isBright = isHovered || isCurrent
        }
    , id = pipelineId
    , databaseID = p.id
    }


instancedPipeline : Params a b -> Concourse.Pipeline -> Views.Pipeline
instancedPipeline =
    pipeline True


instancedPipelineText : Concourse.Pipeline -> String
instancedPipelineText p =
    if Dict.isEmpty p.instanceVars then
        "{}"

    else
        p.instanceVars
            |> Dict.toList
            |> List.concatMap
                (\( k, v ) ->
                    Concourse.flattenJson k v
                        |> List.map (\( key, val ) -> key ++ ":" ++ val)
                )
            |> String.join ","


regularPipeline : Params a b -> Concourse.Pipeline -> Views.Pipeline
regularPipeline =
    pipeline False


regularPipelineText : Concourse.Pipeline -> String
regularPipelineText p =
    p.name
