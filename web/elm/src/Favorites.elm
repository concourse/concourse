module Favorites exposing (Model, handleDelivery, isInstanceGroupFavorited, isPipelineFavorited, update)

import Concourse
import EffectTransformer exposing (ET)
import Json.Encode
import Message.Callback exposing (Callback(..))
import Message.Effects as Effects
import Message.Message exposing (DomID(..), Message(..))
import Message.Subscription exposing (Delivery(..))
import Set exposing (Set)


type alias Model m =
    { m
        | favoritedPipelines : Set Concourse.DatabaseID
        , favoritedInstanceGroups : Set ( Concourse.TeamName, Concourse.PipelineName )
    }


update : Message -> ET (Model m)
update message ( model, effects ) =
    let
        toggle element set =
            if Set.member element set then
                Set.remove element set

            else
                Set.insert element set

        toggleFavoritePipeline pipelineID =
            let
                favoritedPipelines =
                    toggle pipelineID model.favoritedPipelines
            in
            ( { model | favoritedPipelines = favoritedPipelines }
            , [ Effects.SaveFavoritedPipelines favoritedPipelines ]
            )

        toggleFavoriteInstanceGroup ig =
            let
                favoritedInstanceGroups =
                    toggle (instanceGroupKey ig) model.favoritedInstanceGroups
            in
            ( { model | favoritedInstanceGroups = favoritedInstanceGroups }
            , [ Effects.SaveFavoritedInstanceGroups favoritedInstanceGroups ]
            )
    in
    case message of
        Click (SideBarFavoritedIcon pipelineID) ->
            toggleFavoritePipeline pipelineID

        Click (PipelineCardFavoritedIcon _ pipelineID) ->
            toggleFavoritePipeline pipelineID

        Click (TopBarFavoritedIcon pipelineID) ->
            toggleFavoritePipeline pipelineID

        Click (InstanceGroupCardFavoritedIcon _ pipelineID) ->
            toggleFavoriteInstanceGroup pipelineID

        _ ->
            ( model, effects )


handleDelivery : Delivery -> ET (Model m)
handleDelivery delivery ( model, effects ) =
    case delivery of
        FavoritedPipelinesReceived (Ok pipelines) ->
            ( { model | favoritedPipelines = pipelines }, effects )

        FavoritedInstanceGroupsReceived (Ok groups) ->
            ( { model | favoritedInstanceGroups = groups }, effects )

        _ ->
            ( model, effects )


isPipelineFavorited :
    { m | favoritedPipelines : Set Concourse.DatabaseID }
    -> { r | id : Concourse.DatabaseID }
    -> Bool
isPipelineFavorited { favoritedPipelines } { id } =
    Set.member id favoritedPipelines


isInstanceGroupFavorited :
    { m | favoritedInstanceGroups : Set ( Concourse.TeamName, Concourse.PipelineName ) }
    -> Concourse.InstanceGroupIdentifier
    -> Bool
isInstanceGroupFavorited { favoritedInstanceGroups } ig =
    Set.member (instanceGroupKey ig) favoritedInstanceGroups


instanceGroupKey : Concourse.InstanceGroupIdentifier -> ( Concourse.TeamName, Concourse.PipelineName )
instanceGroupKey { teamName, name } =
    ( teamName, name )
