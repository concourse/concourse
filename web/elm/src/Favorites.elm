module Favorites exposing (Model, handleDelivery, isPipelineFavorited, update)

import Concourse
import EffectTransformer exposing (ET)
import Message.Callback exposing (Callback(..))
import Message.Effects as Effects
import Message.Message exposing (DomID(..), Message(..))
import Message.Subscription exposing (Delivery(..))
import Set exposing (Set)


type alias Model m =
    { m
        | favoritedPipelines : Set Concourse.DatabaseID
    }


update : Message -> ET (Model m)
update message ( model, effects ) =
    let
        toggle element set =
            if Set.member element set then
                Set.remove element set

            else
                Set.insert element set

        toggleFavorite pipelineID =
            let
                favoritedPipelines =
                    toggle pipelineID model.favoritedPipelines
            in
            ( { model | favoritedPipelines = favoritedPipelines }
            , [ Effects.SaveFavoritedPipelines <| favoritedPipelines ]
            )
    in
    case message of
        Click (SideBarFavoritedIcon pipelineID) ->
            toggleFavorite pipelineID

        Click (PipelineCardFavoritedIcon _ pipelineID) ->
            toggleFavorite pipelineID

        Click (TopBarFavoritedIcon pipelineID) ->
            toggleFavorite pipelineID

        _ ->
            ( model, effects )


handleDelivery : Delivery -> ET (Model m)
handleDelivery delivery ( model, effects ) =
    case delivery of
        FavoritedPipelinesReceived (Ok pipelines) ->
            ( { model | favoritedPipelines = pipelines }, effects )

        _ ->
            ( model, effects )


isPipelineFavorited : Model m -> { r | id : Concourse.DatabaseID } -> Bool
isPipelineFavorited { favoritedPipelines } { id } =
    Set.member id favoritedPipelines
