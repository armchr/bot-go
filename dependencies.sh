docker run \
-v ~/mariadb_data:/var/lib/mysql  \
-e MARIADB_ROOT_PASSWORD=armchair \
-v ~/mariadb_data:/var/lib/mysql  \
-p 3306:3306 \
-d mariadb:latest

docker run -d \
    --name neo4j \
    -p 7474:7474 -p 7687:7687 \
    -v ~/neo4j/data:/data \
    -v ~/neo4j/logs:/logs \
    -v ~/neo4j/plugins:/plugins \
    #Have to neo4j password has to be set and different from default  password"neo4j"
    -e NEO4J_AUTH=neo4j/armchair \ 
    neo4j:5.13

docker run -d -p 6333:6333 -p 6334:6334 \
      -v "~/qdrant_storage:/qdrant/storage" \
      qdrant/qdrant

# https://github.com/qdrant/qdrant-web-ui.git
# npm start
#
# docker exec -it $(docker ps --filter "publish=3306" --format "{{.Names}}") mariadb -u root -parmchair armchair
