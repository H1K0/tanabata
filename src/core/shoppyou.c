#include <malloc.h>
#include <string.h>
#include <time.h>

#include "../../include/core.h"

int shoppyou_init(Shoppyou *shoppyou) {
    uint64_t timestamp = time(NULL);
    shoppyou->created_ts = timestamp;
    shoppyou->modified_ts = timestamp;
    shoppyou->size = 0;
    shoppyou->removed_cnt = 0;
    shoppyou->contents = NULL;
    shoppyou->file = NULL;
    return 0;
}

int shoppyou_free(Shoppyou *shoppyou) {
    free(shoppyou->contents);
    if (shoppyou->file != NULL) {
        fclose(shoppyou->file);
    }
    return 0;
}

int shoppyou_weed(Shoppyou *shoppyou) {
    if (shoppyou->removed_cnt == 0) {
        return 0;
    }
    uint64_t weeded_size = shoppyou->size - shoppyou->removed_cnt;
    for (uint64_t i = 0, shift = 0; i < shoppyou->size; i++) {
        if (shoppyou->contents[i].sasa_id != 0 && shoppyou->contents[i].tanzaku_id != 0) {
            shoppyou->contents[i - shift] = shoppyou->contents[i];
        } else {
            shift++;
        }
    }
    shoppyou->size = weeded_size;
    shoppyou->removed_cnt = 0;
    shoppyou->contents = realloc(shoppyou->contents, shoppyou->size * sizeof(Kazari));
    return 0;
}

int shoppyou_load(Shoppyou *shoppyou) {
    if (shoppyou->file == NULL) {
        fprintf(stderr, "Failed to load shoppyou: file not specified\n");
        return 1;
    }
    uint16_t signature[4];
    rewind(shoppyou->file);
    fread(signature, 2, 4, shoppyou->file);
    if (memcmp(signature, SHOPPYOU_SIG, 8) != 0) {
        fprintf(stderr, "Failed to load shoppyou: invalid signature\n");
        return 1;
    }
    fread(&shoppyou->created_ts, 8, 1, shoppyou->file);
    fread(&shoppyou->modified_ts, 8, 1, shoppyou->file);
    fread(&shoppyou->size, 8, 1, shoppyou->file);
    shoppyou->removed_cnt = 0;
    shoppyou->contents = malloc(shoppyou->size * sizeof(Kazari));
    for (uint64_t i = 0; i < shoppyou->size; i++) {
        fread(&shoppyou->contents[i].created_ts, 8, 1, shoppyou->file);
        fread(&shoppyou->contents[i].sasa_id, 8, 1, shoppyou->file);
        fread(&shoppyou->contents[i].tanzaku_id, 8, 1, shoppyou->file);
    }
    return 0;
}

int shoppyou_save(Shoppyou *shoppyou) {
    if (shoppyou->file == NULL) {
        fprintf(stderr, "Failed to save shoppyou: file not specified\n");
        return 1;
    }
    if (shoppyou_weed(shoppyou) != 0) {
        fprintf(stderr, "Failed to save shoppyou: failed to weed shoppyou\n");
        return 1;
    }
    rewind(shoppyou->file);
    fwrite(SHOPPYOU_SIG, 2, 4, shoppyou->file);
    fwrite(&shoppyou->created_ts, 8, 1, shoppyou->file);
    fwrite(&shoppyou->modified_ts, 8, 1, shoppyou->file);
    fwrite(&shoppyou->size, 8, 1, shoppyou->file);
    fflush(shoppyou->file);
    for (uint64_t i = 0; i < shoppyou->size; i++) {
        fwrite(&shoppyou->contents[i].created_ts, 8, 1, shoppyou->file);
        fwrite(&shoppyou->contents[i].sasa_id, 8, 1, shoppyou->file);
        fwrite(&shoppyou->contents[i].tanzaku_id, 8, 1, shoppyou->file);
    }
    fflush(shoppyou->file);
    return 0;
}

int shoppyou_open(Shoppyou *shoppyou, const char *path) {
    shoppyou->file = fopen(path, "r+b");
    if (shoppyou->file == NULL) {
        fprintf(stderr, "Failed to dump shoppyou: failed to open file '%s'\n", path);
        return 1;
    }
    return shoppyou_load(shoppyou);
}

int shoppyou_dump(Shoppyou *shoppyou, const char *path) {
    shoppyou->file = fopen(path, "w+b");
    if (shoppyou->file == NULL) {
        fprintf(stderr, "Failed to dump shoppyou: failed to open file '%s'\n", path);
        return 1;
    }
    return shoppyou_save(shoppyou);
}

int kazari_add(Shoppyou *shoppyou, uint64_t sasa_id, uint64_t tanzaku_id) {
    if (shoppyou->size == -1) {
        fprintf(stderr, "Failed to add kazari: shoppyou is full\n");
        return 1;
    }
    Kazari newbie;
    newbie.created_ts = time(NULL);
    newbie.sasa_id = sasa_id;
    newbie.tanzaku_id = tanzaku_id;
    shoppyou->size++;
    shoppyou->contents = realloc(shoppyou->contents, shoppyou->size * sizeof(Kazari));
    shoppyou->contents[shoppyou->size - 1] = newbie;
    shoppyou->modified_ts = newbie.created_ts;
    return 0;
}

int kazari_rem(Shoppyou *shoppyou, uint64_t sasa_id, uint64_t tanzaku_id) {
    for (uint64_t i = 0; i < shoppyou->size; i++) {
        if (shoppyou->contents[i].sasa_id == sasa_id && shoppyou->contents[i].tanzaku_id == tanzaku_id) {
            shoppyou->modified_ts = time(NULL);
            shoppyou->contents[i].sasa_id = 0;
            shoppyou->removed_cnt++;
            return 0;
        }
    }
    fprintf(stderr, "Failed to remove kazari: target kazari does not exist or is already removed\n");
    return 1;
}
